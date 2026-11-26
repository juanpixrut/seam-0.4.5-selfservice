// seam-admin is a command-line tool for managing Seam AMS users and permissions.
//
// Usage:
//
//	seam-admin <command> [options]
//
// Commands:
//
//	user create     Create a new user
//	user list       List users
//	user get        Get user details
//	user disable    Disable a user account
//	user enable     Enable a user account
//	user set-role   Change a user's role
//	user set-password  Set a user's password (admin reset)
//	user delete     Delete a user
//
// Examples:
//
//	seam-admin user create --email=admin@example.com --name="Admin User" --role=admin
//	seam-admin user list --role=admin
//	seam-admin user disable --email=user@example.com
//	seam-admin user set-role --email=user@example.com --role=manager
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	appversion "github.com/ha1tch/seam-ui/internal/version"
)

// Version information (commit and buildDate set via ldflags)
var (
	version   = appversion.Version
	commit    = "dev"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("seam-admin %s (commit: %s, built: %s)\n", version, commit, buildDate)
	case "help", "--help", "-h":
		printUsage()
	case "user":
		if len(os.Args) < 3 {
			printUserUsage()
			os.Exit(1)
		}
		handleUserCommand(os.Args[2], os.Args[3:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`seam-admin - Seam AMS Administration CLI

Usage:
  seam-admin <command> [options]

Commands:
  user        Manage users
  version     Show version information
  help        Show this help

Environment:
  SEAM_XOLU_URL  XOLU server URL (default: http://localhost:9090)

Examples:
  seam-admin user create --email=admin@example.com --name="Admin" --role=admin
  seam-admin user list
  seam-admin user disable --email=user@example.com

Run 'seam-admin <command> help' for more information on a command.`)
}

func printUserUsage() {
	fmt.Println(`seam-admin user - Manage users

Usage:
  seam-admin user <subcommand> [options]

Subcommands:
  create        Create a new user
  list          List all users
  get           Get user details
  disable       Disable a user account
  enable        Enable a user account
  set-role      Change a user's role
  set-password  Reset a user's password
  delete        Delete a user

Options:
  --email       User email address
  --name        User display name
  --role        User role (developer, admin, manager, operator, viewer)
  --tenant      Tenant ID (default: 1)
  --active      Filter by active status (true/false)

Examples:
  seam-admin user create --email=admin@example.com --name="Admin" --role=admin
  seam-admin user create --email=user@example.com --name="User" --role=operator --tenant=2
  seam-admin user list --role=admin
  seam-admin user disable --email=user@example.com
  seam-admin user set-role --email=user@example.com --role=manager
  seam-admin user set-password --email=user@example.com`)
}

func handleUserCommand(subCmd string, args []string) {
	// Parse flags
	flags := parseFlags(args)

	// Get XOLU URL from environment
	xoluURL := os.Getenv("SEAM_XOLU_URL")
	if xoluURL == "" {
		xoluURL = "http://localhost:9090"
	}

	// Create a simple XOLU client for the CLI
	client := &CLIXOLUClient{baseURL: xoluURL}
	ctx := context.Background()

	switch subCmd {
	case "create":
		userCreate(ctx, client, flags)
	case "list":
		userList(ctx, client, flags)
	case "get":
		userGet(ctx, client, flags)
	case "disable":
		userDisable(ctx, client, flags)
	case "enable":
		userEnable(ctx, client, flags)
	case "set-role":
		userSetRole(ctx, client, flags)
	case "set-password":
		userSetPassword(ctx, client, flags)
	case "delete":
		userDelete(ctx, client, flags)
	case "help", "--help", "-h":
		printUserUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown user subcommand: %s\n\n", subCmd)
		printUserUsage()
		os.Exit(1)
	}
}

func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg[2:], "=", 2)
			if len(parts) == 2 {
				flags[parts[0]] = parts[1]
			} else {
				flags[parts[0]] = "true"
			}
		}
	}
	return flags
}

func userCreate(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	email := flags["email"]
	name := flags["name"]
	role := flags["role"]
	tenantIDStr := flags["tenant"]

	if email == "" {
		fmt.Print("Email: ")
		line, err := stdinReader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			fmt.Fprintln(os.Stderr, "Error: email is required")
			os.Exit(1)
		}
		email = strings.TrimSpace(line)
	}
	if name == "" {
		fmt.Print("Name:  ")
		line, err := stdinReader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			fmt.Fprintln(os.Stderr, "Error: name is required")
			os.Exit(1)
		}
		name = strings.TrimSpace(line)
	}
	if role == "" {
		role = "viewer"
	}

	// Parse tenant ID (default to 1)
	tenantID := int64(1)
	if tenantIDStr != "" {
		var err error
		tenantID, err = strconv.ParseInt(tenantIDStr, 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid tenant ID '%s'\n", tenantIDStr)
			os.Exit(1)
		}
	}

	// Validate role
	validRoles := map[string]bool{"developer": true, "admin": true, "manager": true, "operator": true, "viewer": true}
	if !validRoles[role] {
		fmt.Fprintf(os.Stderr, "Error: invalid role '%s'. Must be one of: developer, admin, manager, operator, viewer\n", role)
		os.Exit(1)
	}

	// Prompt for password
	password := promptPassword("Enter password: ")
	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "Error: password must be at least 8 characters")
		os.Exit(1)
	}

	confirmPassword := promptPassword("Confirm password: ")
	if password != confirmPassword {
		fmt.Fprintln(os.Stderr, "Error: passwords do not match")
		os.Exit(1)
	}

	// Hash password with bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
		os.Exit(1)
	}

	// Create user
	user := map[string]interface{}{
		"email":         email,
		"name":          name,
		"role":          role,
		"tenant_id":     tenantID,
		"active":        true,
		"password_hash": string(hashedPassword),
		"created_at":    time.Now().UTC().Format(time.RFC3339),
	}

	id, err := client.Create(ctx, "users", user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created user %s (ID: %d) with role %s in tenant %d\n", email, id, role, tenantID)
}

func userList(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	role := flags["role"]
	activeStr := flags["active"]

	query := "SELECT * FROM users ORDER BY name ASC"
	if role != "" {
		query = fmt.Sprintf("SELECT * FROM users WHERE role = '%s' ORDER BY name ASC", role)
	}
	if activeStr != "" {
		active := activeStr == "true"
		if role != "" {
			query = fmt.Sprintf("SELECT * FROM users WHERE role = '%s' AND active = %t ORDER BY name ASC", role, active)
		} else {
			query = fmt.Sprintf("SELECT * FROM users WHERE active = %t ORDER BY name ASC", active)
		}
	}

	result, err := client.OQL(ctx, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing users: %v\n", err)
		os.Exit(1)
	}

	if len(result) == 0 {
		fmt.Println("No users found")
		return
	}

	// The columns are as wide as what they hold (table.go): nothing is cut,
	// so an email can be copied whole into the next command.
	rows := make([][]string, 0, len(result))
	for _, row := range result {
		activeStr := "Yes"
		if !toBoolFromInterface(row["active"]) {
			activeStr = "No"
		}
		rows = append(rows, []string{
			strconv.FormatInt(toInt64FromInterface(row["id"]), 10),
			toStringFromInterface(row["name"]),
			toStringFromInterface(row["email"]),
			toStringFromInterface(row["role"]),
			activeStr,
		})
	}
	printTable(os.Stdout, []string{"ID", "NAME", "EMAIL", "ROLE", "ACTIVE"}, rows)

	fmt.Printf("\nTotal: %d users\n", len(result))
}

func userGet(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	email := flags["email"]
	if email == "" {
		fmt.Fprintln(os.Stderr, "Error: --email is required")
		os.Exit(1)
	}

	user, err := client.GetByEmail(ctx, "users", email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: user not found\n")
		os.Exit(1)
	}

	fmt.Printf("ID:         %d\n", toInt64FromInterface(user["id"]))
	fmt.Printf("Email:      %s\n", toStringFromInterface(user["email"]))
	fmt.Printf("Name:       %s\n", toStringFromInterface(user["name"]))
	fmt.Printf("Role:       %s\n", toStringFromInterface(user["role"]))
	fmt.Printf("Active:     %t\n", toBoolFromInterface(user["active"]))
	fmt.Printf("Created:    %s\n", toStringFromInterface(user["created_at"]))
	if lastLogin := toStringFromInterface(user["last_login_at"]); lastLogin != "" {
		fmt.Printf("Last Login: %s\n", lastLogin)
	}
}

func userDisable(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	email := flags["email"]
	if email == "" {
		fmt.Fprintln(os.Stderr, "Error: --email is required")
		os.Exit(1)
	}

	user, err := client.GetByEmail(ctx, "users", email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: user not found\n")
		os.Exit(1)
	}

	id := toInt64FromInterface(user["id"])
	err = client.Update(ctx, "users", id, map[string]interface{}{
		"active":     false,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error disabling user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Disabled user %s\n", email)
}

func userEnable(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	email := flags["email"]
	if email == "" {
		fmt.Fprintln(os.Stderr, "Error: --email is required")
		os.Exit(1)
	}

	user, err := client.GetByEmail(ctx, "users", email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: user not found\n")
		os.Exit(1)
	}

	id := toInt64FromInterface(user["id"])
	err = client.Update(ctx, "users", id, map[string]interface{}{
		"active":     true,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Enabled user %s\n", email)
}

func userSetRole(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	email := flags["email"]
	role := flags["role"]

	if email == "" {
		fmt.Fprintln(os.Stderr, "Error: --email is required")
		os.Exit(1)
	}
	if role == "" {
		fmt.Fprintln(os.Stderr, "Error: --role is required")
		os.Exit(1)
	}

	validRoles := map[string]bool{"developer": true, "admin": true, "manager": true, "operator": true, "viewer": true}
	if !validRoles[role] {
		fmt.Fprintf(os.Stderr, "Error: invalid role '%s'. Must be one of: developer, admin, manager, operator, viewer\n", role)
		os.Exit(1)
	}

	user, err := client.GetByEmail(ctx, "users", email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: user not found\n")
		os.Exit(1)
	}

	oldRole := toStringFromInterface(user["role"])
	id := toInt64FromInterface(user["id"])

	err = client.Update(ctx, "users", id, map[string]interface{}{
		"role":       role,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating role: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Changed role for %s: %s -> %s\n", email, oldRole, role)
}

func userSetPassword(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	email := flags["email"]
	if email == "" {
		fmt.Fprintln(os.Stderr, "Error: --email is required")
		os.Exit(1)
	}

	user, err := client.GetByEmail(ctx, "users", email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: user not found\n")
		os.Exit(1)
	}

	password := promptPassword("Enter new password: ")
	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "Error: password must be at least 8 characters")
		os.Exit(1)
	}

	confirmPassword := promptPassword("Confirm new password: ")
	if password != confirmPassword {
		fmt.Fprintln(os.Stderr, "Error: passwords do not match")
		os.Exit(1)
	}

	// Hash password with bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
		os.Exit(1)
	}

	id := toInt64FromInterface(user["id"])
	err = client.Update(ctx, "users", id, map[string]interface{}{
		"password_hash": string(hashedPassword),
		"updated_at":    time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting password: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Password updated for %s\n", email)
}

func userDelete(ctx context.Context, client *CLIXOLUClient, flags map[string]string) {
	email := flags["email"]
	if email == "" {
		fmt.Fprintln(os.Stderr, "Error: --email is required")
		os.Exit(1)
	}

	user, err := client.GetByEmail(ctx, "users", email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: user not found\n")
		os.Exit(1)
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete user '%s' (%s)? [y/N]: ",
		toStringFromInterface(user["name"]), email)

	response, _ := stdinReader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("Aborted")
		return
	}

	id := toInt64FromInterface(user["id"])
	err = client.Delete(ctx, "users", id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted user %s\n", email)
}

// stdinReader is a single shared reader over os.Stdin.
// Creating a new bufio.Reader per call causes the second call to get EOF
// because the first reader buffers ahead and consumes the pipe.
var stdinReader = bufio.NewReader(os.Stdin)

func promptPassword(prompt string) string {
	fmt.Print(prompt)
	password, err := stdinReader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}
	return strings.TrimSpace(password)
}

// Type conversion helpers
func toInt64FromInterface(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func toStringFromInterface(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toBoolFromInterface(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

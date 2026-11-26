// The single global this whole engine exposes. Every module-level
// variable and function that previously lived as a bare global is now
// a property of this one object -- nothing else is declared at the
// top level of this script.
var SEAM_EDITOR = {};

SEAM_EDITOR.Link = function(a, b) {
	this.nodeA = a;
	this.nodeB = b;
	this.text = '';
	this.lineAngleAdjust = 0; // added to the label's angle only when the link is a straight line

	// The link's own curvature handle, stored relative to the straight
	// line between the two nodes rather than as an absolute point --
	// so it stays correctly placed if either node moves.
	this.parallelPart = 0.5;    // 0..1 position along the straight nodeA->nodeB line
	this.perpendicularPart = 0; // pixels off that line; 0 means "straight, no curve"
}

// Converts the link's own (parallelPart, perpendicularPart) coordinates
// -- relative to the straight nodeA->nodeB line -- into an absolute
// point: the actual position of the draggable curvature handle.
SEAM_EDITOR.Link.prototype.getAnchorPoint = function() {
	var deltaX = this.nodeB.x - this.nodeA.x;
	var deltaY = this.nodeB.y - this.nodeA.y;
	var lineLength = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
	var anchorPoint = {
		x: this.nodeA.x + deltaX * this.parallelPart - deltaY * this.perpendicularPart / lineLength,
		y: this.nodeA.y + deltaY * this.parallelPart + deltaX * this.perpendicularPart / lineLength
	};
	return anchorPoint;
};

SEAM_EDITOR.Link.prototype.setAnchorPoint = function(x, y) {
	var deltaX = this.nodeB.x - this.nodeA.x;
	var deltaY = this.nodeB.y - this.nodeA.y;
	var lineLength = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
	this.parallelPart = (deltaX * (x - this.nodeA.x) + deltaY * (y - this.nodeA.y)) / (lineLength * lineLength);
	this.perpendicularPart = (deltaX * (y - this.nodeA.y) - deltaY * (x - this.nodeA.x)) / lineLength;
	// Snap back to a straight line when the drag is close enough to it.
	// lineAngleAdjust remembers which side of the line the drag was on
	// at the moment of snapping, so the label doesn't jump to the wrong
	// side once the curve flattens out.
	if(this.parallelPart > 0 && this.parallelPart < 1 && Math.abs(this.perpendicularPart) < SEAM_EDITOR.snapToPadding) {
		this.lineAngleAdjust = (this.perpendicularPart < 0) * Math.PI;
		this.perpendicularPart = 0;
	}
};

// The core geometry. A link with no curvature (perpendicularPart == 0)
// is a straight line between the two nodes' own circle boundaries --
// not their centers, so the line visually starts/ends exactly at each
// node's edge. A curved link passes through both node centers AND the
// dragged anchor point, so its shape is the unique circle through all
// three (circleFromThreePoints); the arc drawn is the portion of that
// circle between the two nodes, with each end angle nudged inward by
// (that node's own radius / the arc's radius) -- the angular width
// that node's own circle "eats into" the arc -- so the arc starts/ends
// at each node's edge the same way the straight-line case does.
SEAM_EDITOR.Link.prototype.getEndPointsAndCircle = function() {
	if(this.perpendicularPart == 0) {
		var straightMidX = (this.nodeA.x + this.nodeB.x) / 2;
		var straightMidY = (this.nodeA.y + this.nodeB.y) / 2;
		var edgeA = this.nodeA.closestPointOnCircle(straightMidX, straightMidY);
		var edgeB = this.nodeB.closestPointOnCircle(straightMidX, straightMidY);
		var straightGeometry = {
			hasCircle: false,
			startX: edgeA.x,
			startY: edgeA.y,
			endX: edgeB.x,
			endY: edgeB.y
		};
		return straightGeometry;
	}
	var anchor = this.getAnchorPoint();
	var arcCircle = SEAM_EDITOR.circleFromThreePoints(this.nodeA.x, this.nodeA.y, this.nodeB.x, this.nodeB.y, anchor.x, anchor.y);
	// Which side of the straight line the anchor was dragged to --
	// determines the arc's drawing direction (canvas's own
	// "anticlockwise" flag) and the sign of the radius-adjustment below.
	var isReversed = (this.perpendicularPart > 0);
	var reverseScale = isReversed ? 1 : -1;
	var radiusA = this.nodeA._r != null ? this.nodeA._r : SEAM_EDITOR.nodeRadius;
	var radiusB = this.nodeB._r != null ? this.nodeB._r : SEAM_EDITOR.nodeRadius;
	var startAngle = Math.atan2(this.nodeA.y - arcCircle.y, this.nodeA.x - arcCircle.x) - reverseScale * radiusA / arcCircle.radius;
	var endAngle = Math.atan2(this.nodeB.y - arcCircle.y, this.nodeB.x - arcCircle.x) + reverseScale * radiusB / arcCircle.radius;
	var curvedGeometry = {
		hasCircle: true,
		startX: arcCircle.x + arcCircle.radius * Math.cos(startAngle),
		startY: arcCircle.y + arcCircle.radius * Math.sin(startAngle),
		endX: arcCircle.x + arcCircle.radius * Math.cos(endAngle),
		endY: arcCircle.y + arcCircle.radius * Math.sin(endAngle),
		startAngle: startAngle,
		endAngle: endAngle,
		circleX: arcCircle.x,
		circleY: arcCircle.y,
		circleRadius: arcCircle.radius,
		reverseScale: reverseScale,
		isReversed: isReversed
	};
	return curvedGeometry;
};

SEAM_EDITOR.Link.prototype.draw = function(c) {
	var geometry = this.getEndPointsAndCircle();
	// draw arc
	c.beginPath();
	SEAM_EDITOR.ifElse(geometry.hasCircle, () => {
		c.arc(geometry.circleX, geometry.circleY, geometry.circleRadius, geometry.startAngle, geometry.endAngle, geometry.isReversed);
	}, () => {
		c.moveTo(geometry.startX, geometry.startY);
		c.lineTo(geometry.endX, geometry.endY);
	});
	c.stroke();
	// draw the head of the arrow
	SEAM_EDITOR.ifElse(geometry.hasCircle, () => {
		SEAM_EDITOR.drawArrow(c, geometry.endX, geometry.endY, geometry.endAngle - geometry.reverseScale * (Math.PI / 2));
	}, () => {
		SEAM_EDITOR.drawArrow(c, geometry.endX, geometry.endY, Math.atan2(geometry.endY - geometry.startY, geometry.endX - geometry.startX));
	});
	// draw the text -- kept as two fully separate branches deliberately:
	// the curved case's label angle includes isReversed's PI flip, the
	// straight case's does NOT include lineAngleAdjust until the final
	// drawText call (the smoothing overrides below intentionally see
	// the un-adjusted angle in that case).
	SEAM_EDITOR.ifElse(geometry.hasCircle, () => {
		var startAngle = geometry.startAngle;
		var endAngle = geometry.endAngle;
		if(endAngle < startAngle) {
			endAngle += Math.PI * 2;
		}
		var textAngle = (startAngle + endAngle) / 2 + geometry.isReversed * Math.PI;
		var textX = geometry.circleX + geometry.circleRadius * Math.cos(textAngle);
		var textY = geometry.circleY + geometry.circleRadius * Math.sin(textAngle);
		SEAM_EDITOR._drawTextSinOverride = SEAM_EDITOR._smoothLabelSide(this, Math.sin(textAngle));
		SEAM_EDITOR._drawTextCosOverride = SEAM_EDITOR._smoothLabelCos(this, Math.cos(textAngle));
		SEAM_EDITOR.drawText(c, this.text, textX, textY, textAngle, SEAM_EDITOR.selectedObject == this);
		SEAM_EDITOR._drawTextSinOverride = null;
		SEAM_EDITOR._drawTextCosOverride = null;
	}, () => {
		var textX = (geometry.startX + geometry.endX) / 2;
		var textY = (geometry.startY + geometry.endY) / 2;
		var textAngle = Math.atan2(geometry.endX - geometry.startX, geometry.startY - geometry.endY);
		SEAM_EDITOR._drawTextSinOverride = SEAM_EDITOR._smoothLabelSide(this, Math.sin(textAngle));
		SEAM_EDITOR._drawTextCosOverride = SEAM_EDITOR._smoothLabelCos(this, Math.cos(textAngle));
		SEAM_EDITOR.drawText(c, this.text, textX, textY, textAngle + this.lineAngleAdjust, SEAM_EDITOR.selectedObject == this);
		SEAM_EDITOR._drawTextSinOverride = null;
		SEAM_EDITOR._drawTextCosOverride = null;
	});
};

SEAM_EDITOR.Link.prototype.getMidpoint = function() {
	var geometry = this.getEndPointsAndCircle();
	return SEAM_EDITOR.ifElse(geometry.hasCircle, () => {
		var startAngle = geometry.startAngle, endAngle = geometry.endAngle;
		if(endAngle < startAngle) {
			endAngle += Math.PI * 2;
		}
		var mid = (startAngle + endAngle) / 2 + geometry.isReversed * Math.PI;
		var curvedMidpoint = { x: geometry.circleX + geometry.circleRadius * Math.cos(mid),
		                       y: geometry.circleY + geometry.circleRadius * Math.sin(mid) };
		return curvedMidpoint;
	}, () => {
		var straightMidpoint = { x: (geometry.startX + geometry.endX) / 2, y: (geometry.startY + geometry.endY) / 2 };
		return straightMidpoint;
	});
};

SEAM_EDITOR.Link.prototype.drawArcOnly = function(c) {
	var geometry = this.getEndPointsAndCircle();
	c.beginPath();
	SEAM_EDITOR.ifElse(geometry.hasCircle, () => {
		c.arc(geometry.circleX, geometry.circleY, geometry.circleRadius, geometry.startAngle, geometry.endAngle, geometry.isReversed);
	}, () => {
		c.moveTo(geometry.startX, geometry.startY);
		c.lineTo(geometry.endX, geometry.endY);
	});
	c.stroke();
};

// Shared by anything that needs to reason about an arc's angular span
// consistently regardless of its draw direction: canvas's own arc()
// takes a boolean "anticlockwise" flag rather than always drawing from
// the smaller angle to the larger one, so start/end need to be
// swapped back into a direction-independent order before doing any
// angle-range math on them -- hit-testing here, and (below, in the
// LaTeX/SVG export code) a target format that has no direction flag
// of its own. Extracted once three separate copies of this same logic
// were found scattered across the file.
SEAM_EDITOR.normalizedArcAngles = function(startAngle, endAngle, isReversed) {
	if(isReversed) {
		var swap = startAngle;
		startAngle = endAngle;
		endAngle = swap;
	}
	if(endAngle < startAngle) {
		endAngle += Math.PI * 2;
	}
	var normalized = { startAngle: startAngle, endAngle: endAngle };
	return normalized;
}

// Runs ifTrueFn() when expression is truthy, otherwise ifFalseFn(), and
// returns whichever one ran -- so `return SEAM_EDITOR.ifElse(...)`
// propagates a value out of the caller exactly the way
// `if (x) { return a; } else { return b; }` would. Callers pass arrow
// functions when the branch needs the enclosing method's own `this`
// (a plain function expression would get its own `this` instead,
// since it's invoked here as a bare call, not as a method call).
SEAM_EDITOR.ifElse = function(expression, ifTrueFn, ifFalseFn) {
	return expression ? ifTrueFn() : ifFalseFn();
};

SEAM_EDITOR.Link.prototype.containsPoint = function(x, y) {
	var geometry = this.getEndPointsAndCircle();
	if(geometry.hasCircle) {
		var dx = x - geometry.circleX;
		var dy = y - geometry.circleY;
		var distance = Math.sqrt(dx*dx + dy*dy) - geometry.circleRadius;
		if(Math.abs(distance) < SEAM_EDITOR.hitTargetPadding) {
			var angle = Math.atan2(dy, dx);
			var normalized = SEAM_EDITOR.normalizedArcAngles(geometry.startAngle, geometry.endAngle, geometry.isReversed);
			var startAngle = normalized.startAngle;
			var endAngle = normalized.endAngle;
			if(angle < startAngle) {
				angle += Math.PI * 2;
			} else if(angle > endAngle) {
				angle -= Math.PI * 2;
			}
			return (angle > startAngle && angle < endAngle);
		}
	} else {
		var dx = geometry.endX - geometry.startX;
		var dy = geometry.endY - geometry.startY;
		var length = Math.sqrt(dx*dx + dy*dy);
		var percent = (dx * (x - geometry.startX) + dy * (y - geometry.startY)) / (length * length);
		var distance = (dx * (y - geometry.startY) - dy * (x - geometry.startX)) / length;
		return (percent > 0 && percent < 1 && Math.abs(distance) < SEAM_EDITOR.hitTargetPadding);
	}
	return false;
};

SEAM_EDITOR.Node = function(x, y) {
	this.x = x;
	this.y = y;
	this.mouseOffsetX = 0;
	this.mouseOffsetY = 0;
	this.isAcceptState = false;
	this.text = '';
	this.textOnly = false;
	// Force-directed layout state (see the force-layout section below).
	// vx/vy default to 0, not null, so a node that's never been through
	// a simulation still behaves correctly if force math ever reads it
	// directly. fx/fy default to null/undefined ("not pinned") --
	// matching d3-force's own convention -- and are only ever set while
	// the user drags a node during an active simulation.
	this.vx = 0;
	this.vy = 0;
}

SEAM_EDITOR.Node.prototype.setMouseStart = function(x, y) {
	this.mouseOffsetX = this.x - x;
	this.mouseOffsetY = this.y - y;
};

SEAM_EDITOR.Node.prototype.setAnchorPoint = function(x, y) {
	this.x = x + this.mouseOffsetX;
	this.y = y + this.mouseOffsetY;
};

SEAM_EDITOR.Node.prototype.draw = function(c) {
	if (this.textOnly) {
		SEAM_EDITOR.drawText(c, this.text, this.x, this.y, null, SEAM_EDITOR.selectedObject == this);
		return;
	}

	// draw the circle
	var r = this._r != null ? this._r : SEAM_EDITOR.nodeRadius;
	if(this === SEAM_EDITOR._growingNode) {
		var _gp = Math.min(SEAM_EDITOR._growT / SEAM_EDITOR._growDur, 1);
		// cubic ease-out
		var _ge = 1 - Math.pow(1 - _gp, 3);
		r = Math.max(1, r * _ge);
	}
	c.beginPath();
	c.arc(this.x, this.y, r, 0, 2 * Math.PI, false);
	// Fill with canvas background so arcs passing behind nodes are hidden
	var savedStroke = c.strokeStyle;
	var savedLabel  = c.fillStyle;   // label colour from drawUsing
	c.fillStyle = SEAM_EDITOR.seamNodeFill;
	c.fill();
	c.strokeStyle = savedStroke;
	c.stroke();
	c.fillStyle = savedLabel;  // restore for drawText

	// draw the text
	SEAM_EDITOR.drawText(c, this.text, this.x, this.y, null, SEAM_EDITOR.selectedObject == this);

	// draw a double circle for an accept state (Irish green accent)
	// Skip during main pass if this node's label is being edited (overlay handles it)
	var _isEditing = (SEAM_EDITOR.selectedObject == this) && SEAM_EDITOR.editingLabel && SEAM_EDITOR._suppressEditBox;
	if(this.isAcceptState && !_isEditing) {
		c.strokeStyle = SEAM_EDITOR.seamAccentColor;
		c.beginPath();
		c.arc(this.x, this.y, r - 6, 0, 2 * Math.PI, false);
		c.stroke();
		c.strokeStyle = savedStroke;
	}
};

SEAM_EDITOR.Node.prototype.closestPointOnCircle = function(x, y) {
	var radius = this._r != null ? this._r : SEAM_EDITOR.nodeRadius;
	var offsetX = x - this.x;
	var offsetY = y - this.y;
	var offsetLength = Math.sqrt(offsetX * offsetX + offsetY * offsetY);
	var closestPoint = {
		x: this.x + offsetX * radius / offsetLength,
		y: this.y + offsetY * radius / offsetLength
	};
	return closestPoint;
};

SEAM_EDITOR.Node.prototype.containsPoint = function(x, y) {
	var radius = this._r != null ? this._r : SEAM_EDITOR.nodeRadius;
	var offsetX = x - this.x;
	var offsetY = y - this.y;
	return offsetX * offsetX + offsetY * offsetY < radius * radius;
};

SEAM_EDITOR.SelfLink = function(node, mouse) {
	this.node = node;
	this.seam_anchorAngle = 0;
	this.seam_mouseOffsetAngle = 0;
	this.text = '';

	if(mouse) {
		this.setAnchorPoint(mouse.x, mouse.y);
	}
}

// Angle from the node's own center to (x, y) -- the shared calculation
// setMouseStart/setAnchorPoint both need.
SEAM_EDITOR.angleFromNode = function(node, x, y) {
	return Math.atan2(y - node.y, x - node.x);
}

SEAM_EDITOR.SelfLink.prototype.setMouseStart = function(x, y) {
	this.seam_mouseOffsetAngle = this.seam_anchorAngle - SEAM_EDITOR.angleFromNode(this.node, x, y);
};

SEAM_EDITOR.SelfLink.prototype.setAnchorPoint = function(x, y) {
	this.seam_anchorAngle = SEAM_EDITOR.angleFromNode(this.node, x, y) + this.seam_mouseOffsetAngle;
	// Snap to the nearest cardinal direction (0/90/180/270 degrees) when
	// close enough -- makes it easy to get a perfectly horizontal or
	// vertical loop without needing pixel-perfect dragging.
	var nearestCardinal = Math.round(this.seam_anchorAngle / (Math.PI / 2)) * (Math.PI / 2);
	if(Math.abs(this.seam_anchorAngle - nearestCardinal) < 0.1) {
		this.seam_anchorAngle = nearestCardinal;
	}
	// Keep in (-pi, pi] -- containsPoint()'s own angle math assumes this range.
	if(this.seam_anchorAngle < -Math.PI) {
		this.seam_anchorAngle += 2 * Math.PI;
	}
	if(this.seam_anchorAngle > Math.PI) {
		this.seam_anchorAngle -= 2 * Math.PI;
	}
};

// The self-loop is drawn as an arc of a small auxiliary circle, offset
// from the node in the seam_anchorAngle direction -- not the node's own
// circle. The arc spans 288 degrees (0.8*pi to either side of
// seam_anchorAngle), deliberately short of a full circle: the 72-degree gap
// faces the node, so the loop reads as "leaving and re-entering" the
// node rather than a closed ring floating beside it.
SEAM_EDITOR.SelfLink.prototype.getEndPointsAndCircle = function() {
	var nodeRadiusHere = this.node._r != null ? this.node._r : SEAM_EDITOR.nodeRadius;
	var loopDistance = 1.5 * nodeRadiusHere;
	var loopRadius = 0.75 * nodeRadiusHere;
	var loopCenterX = this.node.x + loopDistance * Math.cos(this.seam_anchorAngle);
	var loopCenterY = this.node.y + loopDistance * Math.sin(this.seam_anchorAngle);
	var arcHalfSpan = Math.PI * 0.8;
	var startAngle = this.seam_anchorAngle - arcHalfSpan;
	var endAngle = this.seam_anchorAngle + arcHalfSpan;
	var loopGeometry = {
		hasCircle: true,
		startX: loopCenterX + loopRadius * Math.cos(startAngle),
		startY: loopCenterY + loopRadius * Math.sin(startAngle),
		endX: loopCenterX + loopRadius * Math.cos(endAngle),
		endY: loopCenterY + loopRadius * Math.sin(endAngle),
		startAngle: startAngle,
		endAngle: endAngle,
		circleX: loopCenterX,
		circleY: loopCenterY,
		circleRadius: loopRadius
	};
	return loopGeometry;
};

SEAM_EDITOR.SelfLink.prototype.draw = function(c) {
	var loop = this.getEndPointsAndCircle();
	c.beginPath();
	c.arc(loop.circleX, loop.circleY, loop.circleRadius, loop.startAngle, loop.endAngle, false);
	c.stroke();
	// Label sits on the far side of the loop from the node.
	var labelX = loop.circleX + loop.circleRadius * Math.cos(this.seam_anchorAngle);
	var labelY = loop.circleY + loop.circleRadius * Math.sin(this.seam_anchorAngle);
	SEAM_EDITOR.drawText(c, this.text, labelX, labelY, this.seam_anchorAngle, SEAM_EDITOR.selectedObject == this);
	// Arrowhead sits where the arc ends, angled to point back toward the node.
	SEAM_EDITOR.drawArrow(c, loop.endX, loop.endY, loop.endAngle + Math.PI * 0.4);
};

SEAM_EDITOR.SelfLink.prototype.getMidpoint = function() {
	var loop = this.getEndPointsAndCircle();
	var midpoint = { x: loop.circleX + loop.circleRadius * Math.cos(this.seam_anchorAngle),
	                 y: loop.circleY + loop.circleRadius * Math.sin(this.seam_anchorAngle) };
	return midpoint;
};

SEAM_EDITOR.SelfLink.prototype.drawArcOnly = function(c) {
	var loop = this.getEndPointsAndCircle();
	c.beginPath();
	c.arc(loop.circleX, loop.circleY, loop.circleRadius, loop.startAngle, loop.endAngle, false);
	c.stroke();
};

SEAM_EDITOR.SelfLink.prototype.containsPoint = function(x, y) {
	var loop = this.getEndPointsAndCircle();
	var dx = x - loop.circleX;
	var dy = y - loop.circleY;
	var distanceFromRing = Math.sqrt(dx * dx + dy * dy) - loop.circleRadius;
	return Math.abs(distanceFromRing) < SEAM_EDITOR.hitTargetPadding;
};

SEAM_EDITOR.StartLink = function(node, start) {
	this.node = node;
	this.seam_deltaX = 0;
	this.seam_deltaY = 0;
	this.text = '';

	if(start) {
		this.setAnchorPoint(start.x, start.y);
	}
}

// The start arrow's tail position is stored as an offset from the
// node's own center, not an absolute point -- so it stays correctly
// placed if the node itself is later moved. Small offsets snap to
// exactly zero on either axis so the arrow can land perfectly
// horizontal or vertical instead of a few pixels off.
SEAM_EDITOR.StartLink.prototype.setAnchorPoint = function(x, y) {
	this.seam_deltaX = x - this.node.x;
	this.seam_deltaY = y - this.node.y;

	if(Math.abs(this.seam_deltaX) < SEAM_EDITOR.snapToPadding) {
		this.seam_deltaX = 0;
	}

	if(Math.abs(this.seam_deltaY) < SEAM_EDITOR.snapToPadding) {
		this.seam_deltaY = 0;
	}
};

// The tail sits wherever the user anchored it; the head sits exactly
// on the node's own boundary circle, at the point closest to the
// tail -- so the arrow always terminates cleanly at the node's edge
// regardless of where the tail is dragged.
SEAM_EDITOR.StartLink.prototype.getEndPoints = function() {
	var tailX = this.node.x + this.seam_deltaX;
	var tailY = this.node.y + this.seam_deltaY;
	var head = this.node.closestPointOnCircle(tailX, tailY);
	var endPoints = {
		startX: tailX,
		startY: tailY,
		endX: head.x,
		endY: head.y
	};
	return endPoints;
};

SEAM_EDITOR.StartLink.prototype.draw = function(c) {
	var line = this.getEndPoints();

	// draw the line
	c.beginPath();
	c.moveTo(line.startX, line.startY);
	c.lineTo(line.endX, line.endY);
	c.stroke();

	// Label sits at the tail end, angled along the line away from the node.
	var labelAngle = Math.atan2(line.startY - line.endY, line.startX - line.endX);
	SEAM_EDITOR.drawText(c, this.text, line.startX, line.startY, labelAngle, SEAM_EDITOR.selectedObject == this);

	// Arrowhead points in the direction from tail to node center.
	SEAM_EDITOR.drawArrow(c, line.endX, line.endY, Math.atan2(-this.seam_deltaY, -this.seam_deltaX));
};

SEAM_EDITOR.StartLink.prototype.getMidpoint = function() {
	var line = this.getEndPoints();
	var midpoint = { x: (line.startX + line.endX) / 2, y: (line.startY + line.endY) / 2 };
	return midpoint;
};

SEAM_EDITOR.StartLink.prototype.drawArcOnly = function(c) {
	var line = this.getEndPoints();
	c.beginPath();
	c.moveTo(line.startX, line.startY);
	c.lineTo(line.endX, line.endY);
	c.stroke();
};

// Standard point-to-segment hit test: project the click onto the
// line's own direction to get a 0..1 position along it, and take the
// perpendicular (signed) distance from the line itself.
SEAM_EDITOR.StartLink.prototype.containsPoint = function(x, y) {
	var line = this.getEndPoints();
	var dx = line.endX - line.startX;
	var dy = line.endY - line.startY;
	var length = Math.sqrt(dx*dx + dy*dy);
	var positionAlongLine = (dx * (x - line.startX) + dy * (y - line.startY)) / (length * length);
	var perpendicularDistance = (dx * (y - line.startY) - dy * (x - line.startX)) / length;
	return (positionAlongLine > 0 && positionAlongLine < 1 && Math.abs(perpendicularDistance) < SEAM_EDITOR.hitTargetPadding);
};

// The transient "rubber band" line shown while the user drags out a
// new link, before release decides whether it becomes a Link,
// SelfLink, or StartLink.
SEAM_EDITOR.TemporaryLink = function(from, to) {
	this.from = from;
	this.to = to;
}

SEAM_EDITOR.TemporaryLink.prototype.draw = function(c) {
	c.beginPath();
	c.moveTo(this.to.x, this.to.y);
	c.lineTo(this.from.x, this.from.y);
	c.stroke();

	var pointingAngle = Math.atan2(this.to.y - this.from.y, this.to.x - this.from.x);
	SEAM_EDITOR.drawArrow(c, this.to.x, this.to.y, pointingAngle);
};

// draw using this instead of a canvas and call toLaTeX() afterward
SEAM_EDITOR.ExportAsLaTeX = function(bounds, monochrome) {
	this.bounds = bounds;
	this._seam_points = [];
	this._texData = '';
	this._scale = 0.1; // to convert pixels to document space (TikZ breaks if the numbers get too big, above 500?)
	this._monochrome = !!monochrome;

	// Convert a CSS colour value to a TikZ-compatible colour string.
	// In monochrome mode always returns 'black'.
	// Named colours ('black', 'white') pass through. CSS hex (#rrggbb) is
	// converted to TikZ inline RGB syntax which requires no extra packages.
	this._tikzColor = function(css) {
		if(this._monochrome) {
			return 'black';
		}
		if(!css || css === 'black') {
			return 'black';
		}
		if(css === 'white') {
			return 'white';
		}
		var m = css.match(/^#([0-9a-fA-F]{6})$/);
		if(m) {
			var h = m[1];
			var r = parseInt(h.slice(0,2), 16);
			var g = parseInt(h.slice(2,4), 16);
			var b = parseInt(h.slice(4,6), 16);
			return 'color={rgb,255:red,' + r + ';green,' + g + ';blue,' + b + '}';
		}
		return 'black'; // fallback for anything unrecognised
	};

	// Fix 1: line width — convert canvas lineWidth (px) to pt at the
	// compound scale (×0.1 _scale, ×0.2 tikzpicture) so strokes match screen.
	// canvas px → TikZ coord unit is ×0.02; then ×2.845 pt/unit → ×0.0569.
	// A small boost keeps lines visible: multiply by 1.8 for visual match.
	this._tikzLineWidth = function() { return '1.2pt'; };

	// Complete mapping of Latin-1 Supplement (U+00A0-U+00FF) to LaTeX equivalents.
	// This covers every non-ASCII character in the ANSI/Latin-1 range.
	// Characters above U+00FF fall back to ? substitution in escapedText.
	this._unicodeMap = {
		0x00A0:'~',
		0x00A1:'!`',
		0x00A2:'\\textcent{}',
		0x00A3:'\\pounds{}',
		0x00A4:'\\textcurrency{}',
		0x00A5:'\\textyen{}',
		0x00A6:'\\textbrokenbar{}',
		0x00A7:'\\textsection{}',
		0x00A8:'\\"{}',
		0x00A9:'\\textcopyright{}',
		0x00AA:'\\textordfeminine{}',
		0x00AB:'\\guillemotleft{}',
		0x00AC:'\\ensuremath{\\neg}',
		0x00AD:'\\-',
		0x00AE:'\\textregistered{}',
		0x00AF:'\\={}',
		0x00B0:'\\textdegree{}',
		0x00B1:'\\ensuremath{\\pm}',
		0x00B2:'\\textsuperscript{2}',
		0x00B3:'\\textsuperscript{3}',
		0x00B4:"\\'{}",
		0x00B5:'\\ensuremath{\\mu}',
		0x00B6:'\\textparagraph{}',
		0x00B7:'\\ensuremath{\\cdot}',
		0x00B8:'\\c{}',
		0x00B9:'\\textsuperscript{1}',
		0x00BA:'\\textordmasculine{}',
		0x00BB:'\\guillemotright{}',
		0x00BC:'\\textonequarter{}',
		0x00BD:'\\textonehalf{}',
		0x00BE:'\\textthreequarters{}',
		0x00BF:'?`',
		0x00C0:'\\`{A}',  0x00C1:"\\'{ A}",  0x00C2:'\\^{A}',  0x00C3:'\\~{A}',
		0x00C4:'\\"{A}',  0x00C5:'\\AA{}',   0x00C6:'\\AE{}',   0x00C7:'\\c{C}',
		0x00C8:'\\`{E}',  0x00C9:"\\'{ E}",  0x00CA:'\\^{E}',  0x00CB:'\\"{E}',
		0x00CC:'\\`{I}',  0x00CD:"\\'{ I}",  0x00CE:'\\^{I}',  0x00CF:'\\"{I}',
		0x00D0:'\\DH{}',  0x00D1:'\\~{N}',   0x00D2:'\\`{O}',  0x00D3:"\\'{ O}",
		0x00D4:'\\^{O}',  0x00D5:'\\~{O}',   0x00D6:'\\"{O}',  0x00D7:'\\ensuremath{\\times}',
		0x00D8:'\\O{}',   0x00D9:'\\`{U}',   0x00DA:"\\'{ U}",  0x00DB:'\\^{U}',
		0x00DC:'\\"{U}',  0x00DD:"\\'{ Y}",  0x00DE:'\\TH{}',  0x00DF:'\\ss{}',
		0x00E0:'\\`{a}',  0x00E1:"\\'{ a}",  0x00E2:'\\^{a}',  0x00E3:'\\~{a}',
		0x00E4:'\\"{a}',  0x00E5:'\\aa{}',   0x00E6:'\\ae{}',   0x00E7:'\\c{c}',
		0x00E8:'\\`{e}',  0x00E9:"\\'{ e}",  0x00EA:'\\^{e}',  0x00EB:'\\"{e}',
		0x00EC:'\\`{\\i}', 0x00ED:"\\'{ \\i}", 0x00EE:'\\^{\\i}', 0x00EF:'\\"{\\i}',
		0x00F0:'\\dh{}',  0x00F1:'\\~{n}',   0x00F2:'\\`{o}',  0x00F3:"\\'{ o}",
		0x00F4:'\\^{o}',  0x00F5:'\\~{o}',   0x00F6:'\\"{o}',  0x00F7:'\\ensuremath{\\div}',
		0x00F8:'\\o{}',   0x00F9:'\\`{u}',   0x00FA:"\\'{ u}",  0x00FB:'\\^{u}',
		0x00FC:'\\"{u}',  0x00FD:"\\'{ y}",  0x00FE:'\\th{}',  0x00FF:'\\"{y}',
		// Greek uppercase (U+0391-U+03A9)
		0x0391:'\\ensuremath{A}',          0x0392:'\\ensuremath{B}',
		0x0393:'\\ensuremath{\\Gamma}',   0x0394:'\\ensuremath{\\Delta}',
		0x0395:'\\ensuremath{E}',           0x0396:'\\ensuremath{Z}',
		0x0397:'\\ensuremath{H}',           0x0398:'\\ensuremath{\\Theta}',
		0x0399:'\\ensuremath{I}',           0x039A:'\\ensuremath{K}',
		0x039B:'\\ensuremath{\\Lambda}',  0x039C:'\\ensuremath{M}',
		0x039D:'\\ensuremath{N}',           0x039E:'\\ensuremath{\\Xi}',
		0x039F:'\\ensuremath{O}',           0x03A0:'\\ensuremath{\\Pi}',
		0x03A1:'\\ensuremath{P}',           0x03A3:'\\ensuremath{\\Sigma}',
		0x03A4:'\\ensuremath{T}',           0x03A5:'\\ensuremath{\\Upsilon}',
		0x03A6:'\\ensuremath{\\Phi}',      0x03A7:'\\ensuremath{X}',
		0x03A8:'\\ensuremath{\\Psi}',      0x03A9:'\\ensuremath{\\Omega}',
		// Greek lowercase (U+03B1-U+03C9)
		0x03B1:'\\ensuremath{\\alpha}',    0x03B2:'\\ensuremath{\\beta}',
		0x03B3:'\\ensuremath{\\gamma}',    0x03B4:'\\ensuremath{\\delta}',
		0x03B5:'\\ensuremath{\\varepsilon}',0x03B6:'\\ensuremath{\\zeta}',
		0x03B7:'\\ensuremath{\\eta}',      0x03B8:'\\ensuremath{\\theta}',
		0x03B9:'\\ensuremath{\\iota}',     0x03BA:'\\ensuremath{\\kappa}',
		0x03BB:'\\ensuremath{\\lambda}',   0x03BC:'\\ensuremath{\\mu}',
		0x03BD:'\\ensuremath{\\nu}',       0x03BE:'\\ensuremath{\\xi}',
		0x03BF:'\\ensuremath{o}',           0x03C0:'\\ensuremath{\\pi}',
		0x03C1:'\\ensuremath{\\rho}',      0x03C2:'\\ensuremath{\\varsigma}',
		0x03C3:'\\ensuremath{\\sigma}',    0x03C4:'\\ensuremath{\\tau}',
		0x03C5:'\\ensuremath{\\upsilon}',  0x03C6:'\\ensuremath{\\varphi}',
		0x03C7:'\\ensuremath{\\chi}',      0x03C8:'\\ensuremath{\\psi}',
		0x03C9:'\\ensuremath{\\omega}',
		// Unicode subscript digits U+2080-U+2089
		0x2080:'\\textsubscript{0}', 0x2081:'\\textsubscript{1}',
		0x2082:'\\textsubscript{2}', 0x2083:'\\textsubscript{3}',
		0x2084:'\\textsubscript{4}', 0x2085:'\\textsubscript{5}',
		0x2086:'\\textsubscript{6}', 0x2087:'\\textsubscript{7}',
		0x2088:'\\textsubscript{8}', 0x2089:'\\textsubscript{9}',
		// Unicode superscript digits U+2070, U+00B9, U+00B2, U+00B3, U+2074-U+2079
		0x2070:'\\textsuperscript{0}', 0x2074:'\\textsuperscript{4}',
		0x2075:'\\textsuperscript{5}', 0x2076:'\\textsuperscript{6}',
		0x2077:'\\textsuperscript{7}', 0x2078:'\\textsuperscript{8}',
		0x2079:'\\textsuperscript{9}',
		// Mathematical operators and symbols
		0x2200:'\\ensuremath{\\forall}',    0x2203:'\\ensuremath{\\exists}',
		0x2204:'\\ensuremath{\\nexists}',   0x2205:'\\ensuremath{\\emptyset}',
		0x2207:'\\ensuremath{\\nabla}',     0x2208:'\\ensuremath{\\in}',
		0x2209:'\\ensuremath{\\notin}',     0x220B:'\\ensuremath{\\ni}',
		0x220F:'\\ensuremath{\\prod}',      0x2211:'\\ensuremath{\\sum}',
		0x2212:'\\ensuremath{-}',            0x2213:'\\ensuremath{\\mp}',
		0x2215:'\\ensuremath{/}',            0x2217:'\\ensuremath{*}',
		0x221A:'\\ensuremath{\\sqrt{}}',    0x221B:'\\ensuremath{\\sqrt[3]{}}',
		0x221E:'\\ensuremath{\\infty}',     0x2202:'\\ensuremath{\\partial}',
		0x2220:'\\ensuremath{\\angle}',     0x2221:'\\ensuremath{\\measuredangle}',
		0x2225:'\\ensuremath{\\parallel}',  0x2226:'\\ensuremath{\\nparallel}',
		0x2227:'\\ensuremath{\\wedge}',     0x2228:'\\ensuremath{\\vee}',
		0x2229:'\\ensuremath{\\cap}',       0x222A:'\\ensuremath{\\cup}',
		0x222B:'\\ensuremath{\\int}',       0x222C:'\\ensuremath{\\iint}',
		0x2236:'\\ensuremath{:}',            0x2237:'\\ensuremath{::}',
		0x223C:'\\ensuremath{\\sim}',       0x2243:'\\ensuremath{\\simeq}',
		0x2245:'\\ensuremath{\\cong}',      0x2248:'\\ensuremath{\\approx}',
		0x224D:'\\ensuremath{\\asymp}',     0x2250:'\\ensuremath{\\doteq}',
		0x2260:'\\ensuremath{\\neq}',       0x2261:'\\ensuremath{\\equiv}',
		0x2262:'\\ensuremath{\\not\\equiv}',0x2264:'\\ensuremath{\\leq}',
		0x2265:'\\ensuremath{\\geq}',       0x226A:'\\ensuremath{\\ll}',
		0x226B:'\\ensuremath{\\gg}',        0x2282:'\\ensuremath{\\subset}',
		0x2283:'\\ensuremath{\\supset}',    0x2286:'\\ensuremath{\\subseteq}',
		0x2287:'\\ensuremath{\\supseteq}',  0x22A5:'\\ensuremath{\\perp}',
		0x22C5:'\\ensuremath{\\cdot}',      0x22EF:'\\ensuremath{\\cdots}',
		// Arrows
		0x2190:'\\ensuremath{\\leftarrow}',  0x2191:'\\ensuremath{\\uparrow}',
		0x2192:'\\ensuremath{\\rightarrow}', 0x2193:'\\ensuremath{\\downarrow}',
		0x2194:'\\ensuremath{\\leftrightarrow}',0x2195:'\\ensuremath{\\updownarrow}',
		0x21D0:'\\ensuremath{\\Leftarrow}',  0x21D1:'\\ensuremath{\\Uparrow}',
		0x21D2:'\\ensuremath{\\Rightarrow}', 0x21D3:'\\ensuremath{\\Downarrow}',
		0x21D4:'\\ensuremath{\\Leftrightarrow}',
		0x21A6:'\\ensuremath{\\mapsto}',     0x21A9:'\\ensuremath{\\hookleftarrow}',
		// Miscellaneous technical
		0x2126:'\\ensuremath{\\Omega}',      0x212B:'\\AA{}',
		0x2032:"\\ensuremath{'}",            0x2033:"\\ensuremath{''}",
		0x2026:'\\ldots{}',
		// Typography
		0x2013:'--',  0x2014:'---',
		0x201C:'``',  0x201D:"''",  0x2018:'`',  0x2019:"'",
	};

	// Scan all node and link labels; collect Unicode chars and build preamble declarations.
	this._buildUnicodeDeclarations = function() {
		var texts = [];
		for(var i = 0; i < SEAM_EDITOR.nodes.length; i++) texts.push(SEAM_EDITOR.nodes[i].text || '');
		for(var i = 0; i < SEAM_EDITOR.links.length; i++) {
			texts.push(SEAM_EDITOR.links[i].text || '');
			if(SEAM_EDITOR.links[i]._id) {
				var lp = SEAM_EDITOR.linkProperties.get(SEAM_EDITOR.links[i]._id) || {};
				if(lp.guard) {
					texts.push(lp.guard);
				}
				if(lp.action) {
					texts.push(lp.action);
				}
			}
		}
		var seen = {};
		var decls = '';
		var allText = texts.join('');
		for(var j = 0; j < allText.length; j++) {
			var cp = allText.codePointAt(j);
			if(cp > 0x7F && cp <= 0xFF && !seen[cp]) {
				seen[cp] = true;
				var hex = cp.toString(16).toUpperCase().padStart(4, '0');
				var mapped = this._unicodeMap[cp];
				if(mapped) {
					decls += '\\DeclareUnicodeCharacter{' + hex + '}{' + mapped + '}\n';
				}
				// Unknown chars: fallthrough to the ?-replacement in escapedText
			}
		}
		return decls;
	};

	this.toLaTeX = function() {
		var unicodeDeclarations = this._buildUnicodeDeclarations();
		return '\\documentclass[10pt]{article}\n' +
			'\\usepackage{tikz}\n' +
			'\\usepackage[utf8]{inputenc}\n' +
			'\\renewcommand{\\familydefault}{\\sfdefault}\n' +
			(unicodeDeclarations ? unicodeDeclarations : '') +
			'\n' +
			'\\begin{document}\n' +
			'\n' +
			'\\begin{center}\n' +
			'\\resizebox{\\textwidth}{!}{%\n' +
			'\\begin{tikzpicture}[scale=0.2]\n' +
			'\\tikzstyle{every node}+=[inner sep=0pt]\n' +
			this._texData +
			'\\end{tikzpicture}%\n' +
			'}\n' +
			'\\end{center}\n' +
			'\n' +
			'\\end{document}\n';
	};

	// Converts a world-space point into this exporter's own scaled,
	// bounds-relative coordinate space: shift so the bounding box's own
	// top-left corner becomes the origin, then scale down by _scale --
	// TikZ's own coordinate system breaks on large numbers, so pixel
	// coordinates need shrinking before they reach any TikZ command.
	this._toLocal = function(x, y) {
		var localPoint = {
			x: (x - this.bounds[0]) * this._scale,
			y: (y - this.bounds[1]) * this._scale
		};
		return localPoint;
	};

	this.beginPath = function() {
		this._seam_points = [];
	};
	this.arc = function(x, y, radius, startAngle, endAngle, isReversed) {
		var local = this._toLocal(x, y);
		x = local.x;
		y = local.y;
		radius *= this._scale;
		var strokeColor = this._tikzColor(this.strokeStyle);
		var strokeWidth = this._tikzLineWidth();
		SEAM_EDITOR.ifElse(endAngle - startAngle == Math.PI * 2, () => {
			this._texData += '\\draw [' + strokeColor + ',line width=' + strokeWidth + '] (' + SEAM_EDITOR.fixed(x, 3) + ',' + SEAM_EDITOR.fixed(-y, 3) + ') circle (' + SEAM_EDITOR.fixed(radius, 3) + ');\n';
		}, () => {
			var normalized = SEAM_EDITOR.normalizedArcAngles(startAngle, endAngle, isReversed);
			startAngle = normalized.startAngle;
			endAngle = normalized.endAngle;
			// TikZ needs the angles to be between -2pi and 2pi or it breaks
			if(Math.min(startAngle, endAngle) < -2*Math.PI) {
				startAngle += 2*Math.PI;
				endAngle += 2*Math.PI;
			} else if(Math.max(startAngle, endAngle) > 2*Math.PI) {
				startAngle -= 2*Math.PI;
				endAngle -= 2*Math.PI;
			}
			startAngle = -startAngle;
			endAngle = -endAngle;
			this._texData += '\\draw [' + strokeColor + ',line width=' + strokeWidth + '] (' + SEAM_EDITOR.fixed(x + radius * Math.cos(startAngle), 3) + ',' + SEAM_EDITOR.fixed(-y + radius * Math.sin(startAngle), 3) + ') arc (' + SEAM_EDITOR.fixed(startAngle * 180 / Math.PI, 5) + ':' + SEAM_EDITOR.fixed(endAngle * 180 / Math.PI, 5) + ':' + SEAM_EDITOR.fixed(radius, 3) + ');\n';
		});
	};
	this.moveTo = this.lineTo = function(x, y) {
		var local = this._toLocal(x, y);
		this._seam_points.push({ x: local.x, y: local.y });
	};
	// stroke() and fill() both emit the same TikZ polyline-through-points
	// syntax, differing only in the command name and where the drawing
	// colour comes from -- pulled into one shared builder rather than
	// keeping two nearly-identical copies of the loop.
	this._buildTikzPolyline = function(command, color) {
		if(this._seam_points.length == 0) {
			return '';
		}
		var out = command + ' [' + color + ']';
		for(var i = 0; i < this._seam_points.length; i++) {
			var point = this._seam_points[i];
			out += (i > 0 ? ' --' : '') + ' (' + SEAM_EDITOR.fixed(point.x, 2) + ',' + SEAM_EDITOR.fixed(-point.y, 2) + ')';
		}
		return out + ';\n';
	};
	this.stroke = function() {
		this._texData += this._buildTikzPolyline('\\draw', this._tikzColor(this.strokeStyle) + ',line width=' + this._tikzLineWidth());
	};
	this.fill = function() {
		// Note: uses strokeStyle rather than fillStyle for the fill
		// colour too, matching the calling code's own convention of
		// setting strokeStyle before either stroking or filling a shape.
		this._texData += this._buildTikzPolyline('\\fill', this._tikzColor(this.strokeStyle));
	};
	this.measureText = function(text) {
		var ctx = SEAM_EDITOR.canvas.getContext('2d');
		ctx.font = SEAM_EDITOR.seamFont;
		return ctx.measureText(text);
	};
	// Positions a label relative to (x, y): centered when angleOrNull is
	// null, or shifted to one side -- right, left, above, or below,
	// whichever the angle points closest to -- when angleOrNull is
	// given (used for link labels, which sit beside the link rather
	// than centered on a point). Everything below the positioning
	// decision is Seam's own LaTeX-escaping and Unicode-substitution
	// logic, unrelated to the original positioning concept.
	this.advancedFillText = function(text, originalText, x, y, angleOrNull) {

		x -= this.bounds[0];
		y -= this.bounds[1];
		if(text.replace(' ', '').length > 0) {
			var nodeParams = '';
			// x and y start off as the center of the text, but will be moved to one side of the box when angleOrNull != null
			if(angleOrNull != null) {
				var labelWidth = this.measureText(text).width;
				var dx = Math.cos(angleOrNull);
				var dy = Math.sin(angleOrNull);
				SEAM_EDITOR.ifElse(Math.abs(dx) > Math.abs(dy), () => {
					SEAM_EDITOR.ifElse(dx > 0, () => {
						nodeParams = '[right] ';
						x -= labelWidth / 2;
					}, () => {
						nodeParams = '[left] ';
						x += labelWidth / 2;
					});
				}, () => {
					SEAM_EDITOR.ifElse(dy > 0, () => {
						nodeParams = '[below] ';
						y -= 10;
					}, () => {
						nodeParams = '[above] ';
						y += 10;
					});
				});
			}
			x *= this._scale;
			y *= this._scale;
			// Fix 2: label colour is always black (labels are dark on a light node)
			// Fix 3: \small reduces font ~17% relative to 10pt base
			// Fix 4: \sffamily\upshape gives sans-serif upright (no math mode)
			// Replace Unicode sub/superscripts before other escaping
			var _subMap = {'₀':'\\textsubscript{0}','₁':'\\textsubscript{1}','₂':'\\textsubscript{2}','₃':'\\textsubscript{3}','₄':'\\textsubscript{4}','₅':'\\textsubscript{5}','₆':'\\textsubscript{6}','₇':'\\textsubscript{7}','₈':'\\textsubscript{8}','₉':'\\textsubscript{9}'};
			var _supMap = {'⁰':'\\textsuperscript{0}','¹':'\\textsuperscript{1}','²':'\\textsuperscript{2}','³':'\\textsuperscript{3}','⁴':'\\textsuperscript{4}','⁵':'\\textsuperscript{5}','⁶':'\\textsuperscript{6}','⁷':'\\textsuperscript{7}','⁸':'\\textsuperscript{8}','⁹':'\\textsuperscript{9}'};
			var escapedText = originalText
				.replace(/[₀₁₂₃₄₅₆₇₈₉]/g, function(c){return _subMap[c]||c;})
				.replace(/[⁰¹²³⁴⁵⁶⁷⁸⁹]/g, function(c){return _supMap[c]||c;})
				.replace(/[^\x00-\xFF]/g, (function(self) { return function(c) {
					var mapped = self._unicodeMap[c.codePointAt(0)];
					return mapped !== undefined ? mapped : '?';
				}; })(this)) // substitute from map, fallback ?
				.replace(/\\/g, '\\textbackslash{}')
				.replace(/{/g, '\\{')
				.replace(/}/g, '\\}')
				.replace(/_/g, '\\_')
				.replace(/\^/g, '\\^{}')
				.replace(/&/g, '\\&')
				.replace(/%/g, '\\%')
				.replace(/\$/g, '\\$')
				.replace(/#/g, '\\#')
				.replace(/~/g, '\\textasciitilde{}')
				.replace(/</g, '\\textless{}')
				.replace(/>/g, '\\textgreater{}')
				.replace(/"/g, "'");
			this._texData += '\\draw [black] (' + SEAM_EDITOR.fixed(x, 2) + ',' + SEAM_EDITOR.fixed(-y, 2) + ') node ' + nodeParams + '{\\fontsize{5.99}{7.2}\\selectfont\\sffamily\\upshape ' + escapedText + '};\n';
		}
	};

	this.translate = this.save = this.restore = this.clearRect = this.scale = function(){};
}

// draw using this instead of a canvas and call toSVG() afterward
SEAM_EDITOR.ExportAsSVG = function(bounds) {
	this.width = bounds[2] - bounds[0];
	this.height = bounds[3] - bounds[1];
	this.bounds = bounds;
	this.fillStyle = 'black';
	this.strokeStyle = 'black';
	this.lineWidth = 1;
	this.font = '12px Arial, sans-serif';
	this._seam_points = [];
	this._seam_svgData = '';
	this._seam_transX = 0;
	this._seam_transY = 0;

	this.toSVG = function() {
		var data = '<?xml version="1.0" standalone="no"?>\n';
		data += '<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">\n\n';
		data += '<svg width="' + this.width + '" height="' + this.height + '" version="1.1" xmlns="http://www.w3.org/2000/svg">\n';
		data += this._seam_svgData;
		data += '</svg>\n';
		return data;
	};

	// Converts a world-space point into this exporter's own local
	// coordinate space: shift so the bounding box's own top-left corner
	// becomes the origin, then apply whatever offset translate() has
	// accumulated. Every drawing method below needs exactly this same
	// conversion before it can record a coordinate -- extracted here
	// once, rather than repeating the same four lines in each one.
	this._toLocal = function(x, y) {
		var localPoint = {
			x: x - this.bounds[0] + this._seam_transX,
			y: y - this.bounds[1] + this._seam_transY
		};
		return localPoint;
	};

	this.beginPath = function() {
		this._seam_points = [];
	};
	this.arc = function(x, y, radius, startAngle, endAngle, isReversed) {
		var local = this._toLocal(x, y);
		x = local.x;
		y = local.y;
		var style = 'stroke="' + this.strokeStyle + '" stroke-width="' + this.lineWidth + '" fill="none"';

		SEAM_EDITOR.ifElse(endAngle - startAngle == Math.PI * 2, () => {
			this._seam_svgData += '\t<ellipse ' + style + ' cx="' + SEAM_EDITOR.fixed(x, 3) + '" cy="' + SEAM_EDITOR.fixed(y, 3) + '" rx="' + SEAM_EDITOR.fixed(radius, 3) + '" ry="' + SEAM_EDITOR.fixed(radius, 3) + '"/>\n';
		}, () => {
			var normalized = SEAM_EDITOR.normalizedArcAngles(startAngle, endAngle, isReversed);
			startAngle = normalized.startAngle;
			endAngle = normalized.endAngle;

			var arcStartX = x + radius * Math.cos(startAngle);
			var arcStartY = y + radius * Math.sin(startAngle);
			var arcEndX = x + radius * Math.cos(endAngle);
			var arcEndY = y + radius * Math.sin(endAngle);
			var useLargeArcFlag = (Math.abs(endAngle - startAngle) > Math.PI);
			var sweepDirectionFlag = 1;

			this._seam_svgData += '\t<path ' + style + ' d="';
			this._seam_svgData += 'M ' + SEAM_EDITOR.fixed(arcStartX, 3) + ',' + SEAM_EDITOR.fixed(arcStartY, 3) + ' '; // startPoint
			this._seam_svgData += 'A ' + SEAM_EDITOR.fixed(radius, 3) + ',' + SEAM_EDITOR.fixed(radius, 3) + ' '; // radii
			this._seam_svgData += '0 '; // value of 0 means perfect circle, others mean ellipse
			this._seam_svgData += +useLargeArcFlag + ' ';
			this._seam_svgData += +sweepDirectionFlag + ' ';
			this._seam_svgData += SEAM_EDITOR.fixed(arcEndX, 3) + ',' + SEAM_EDITOR.fixed(arcEndY, 3); // endPoint
			this._seam_svgData += '"/>\n';
		});
	};
	this.moveTo = this.lineTo = function(x, y) {
		var local = this._toLocal(x, y);
		this._seam_points.push({ x: local.x, y: local.y });
	};
	this.stroke = function() {
		if(this._seam_points.length == 0) return;
		this._seam_svgData += '\t<polygon stroke="' + this.strokeStyle + '" stroke-width="' + this.lineWidth + '" points="';
		for(var i = 0; i < this._seam_points.length; i++) {
			this._seam_svgData += (i > 0 ? ' ' : '') + SEAM_EDITOR.fixed(this._seam_points[i].x, 3) + ',' + SEAM_EDITOR.fixed(this._seam_points[i].y, 3);
		}
		this._seam_svgData += '"/>\n';
	};
	this.fill = function() {
		if(this._seam_points.length == 0) return;
		this._seam_svgData += '\t<polygon fill="' + this.fillStyle + '" stroke-width="' + this.lineWidth + '" points="';
		for(var i = 0; i < this._seam_points.length; i++) {
			this._seam_svgData += (i > 0 ? ' ' : '') + SEAM_EDITOR.fixed(this._seam_points[i].x, 3) + ',' + SEAM_EDITOR.fixed(this._seam_points[i].y, 3);
		}
		this._seam_svgData += '"/>\n';
	};
	this.measureText = function(text) {
		var c = SEAM_EDITOR.canvas.getContext('2d');
		c.font = SEAM_EDITOR.seamFont;
		return c.measureText(text);
	};
	this.fillText = function(text, x, y) {
		var local = this._toLocal(x, y);
		x = local.x;
		y = local.y;
		if(text.replace(' ', '').length > 0) {
			this._seam_svgData += '\t<text x="' + SEAM_EDITOR.fixed(x, 3) + '" y="' + SEAM_EDITOR.fixed(y, 3) + '" font-family="Times New Roman" font-size="20">' + SEAM_EDITOR.textToXML(text) + '</text>\n';
		}
	};
	this.translate = function(x, y) {
		this._seam_transX = x;
		this._seam_transY = y;
	};
	this.scale = function() {}; // no-op: viewport reset to k=1 before export

	this.advancedFillText = function(text, originalText, x, y, angleOrNull) {
		var local = this._toLocal(x, y);
		x = local.x;
		y = local.y;
		if(text.replace(' ', '').length === 0) return;
		// Parse font size from seamFont (e.g. "12px system-ui, ...")
		var fontSize = 12;
		if(typeof SEAM_EDITOR.seamFont === 'string') {
			var fm = SEAM_EDITOR.seamFont.match(/^(\d+(?:\.\d+)?)px/);
			if(fm) fontSize = parseFloat(fm[1]);
		}
		// Use seamActiveLabelColor for the fill, falling back to strokeStyle
		var fill = (typeof SEAM_EDITOR.seamActiveLabelColor === 'string' && SEAM_EDITOR.seamActiveLabelColor)
			? SEAM_EDITOR.seamActiveLabelColor : this.strokeStyle;
		this._seam_svgData += '\t<text x="' + SEAM_EDITOR.fixed(x, 3) + '" y="' + SEAM_EDITOR.fixed(y + fontSize * 0.35, 3) + '"' +
			' font-family="sans-serif" font-size="' + fontSize + '"' +
			' text-anchor="middle" fill="' + fill + '">' + SEAM_EDITOR.textToXML(text) + '</text>\n';
	};

	this.save = this.restore = this.clearRect = function(){};
}

SEAM_EDITOR.greekLetterNames = [ 'Alpha', 'Beta', 'Gamma', 'Delta', 'Epsilon', 'Zeta', 'Eta', 'Theta', 'Iota', 'Kappa', 'Lambda', 'Mu', 'Nu', 'Xi', 'Omicron', 'Pi', 'Rho', 'Sigma', 'Tau', 'Upsilon', 'Phi', 'Chi', 'Psi', 'Omega' ];

// Named symbol shortcuts that aren't Greek letters -- kept as their own
// explicit map rather than piggy-backing on the Greek-letter array via
// early-exit special cases inside that loop, which is how this used to
// be structured.
SEAM_EDITOR.namedSymbolShortcuts = { emptyset: 8709, rightarrow: 8594, leftarrow: 8592 };

SEAM_EDITOR.convertLatexShortcuts = function(text) {
	for(var name in SEAM_EDITOR.namedSymbolShortcuts) {
		text = text.replace(new RegExp('\\\\' + name, 'g'), String.fromCharCode(SEAM_EDITOR.namedSymbolShortcuts[name]));
	}

	// Greek letters, upper and lower case (\Alpha and \alpha).
	for(var i = 0; i < SEAM_EDITOR.greekLetterNames.length; i++) {
		var name = SEAM_EDITOR.greekLetterNames[i];
		// Unicode's uppercase Greek block reserves U+03A2 (no character
		// assigned there -- historically a final-sigma slot in some
		// older encodings, left undefined here), so every letter from
		// Sigma (index 17) onward needs its code point nudged up by one
		// to skip that gap. Same gap, same nudge, in the lowercase block.
		var codePointOffset = i + (i >= 17 ? 1 : 0);
		text = text.replace(new RegExp('\\\\' + name, 'g'), String.fromCharCode(913 + codePointOffset));
		text = text.replace(new RegExp('\\\\' + name.toLowerCase(), 'g'), String.fromCharCode(945 + codePointOffset));
	}

	// Numeric subscripts (q_0 becomes q with a subscript zero, etc.)
	for(var digit = 0; digit < 10; digit++) {
		text = text.replace(new RegExp('_' + digit, 'g'), String.fromCharCode(8320 + digit));
	}

	return text;
}

// Escapes text for safe inclusion as XML/SVG content: the three
// characters XML syntax itself uses (&, <, >) become entities, and
// anything outside printable ASCII becomes a numeric character
// reference -- safe regardless of the document's own declared
// encoding, since numeric references are just digits.
SEAM_EDITOR.textToXML = function(text) {
	text = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
	var escaped = '';
	for(var i = 0; i < text.length; i++) {
		var charCode = text.charCodeAt(i);
		if(charCode >= 0x20 && charCode <= 0x7E) {
			escaped += text[i];
		} else {
			escaped += '&#' + charCode + ';';
		}
	}
	return escaped;
}

// Draws a filled triangular arrowhead with its tip at (x, y), pointing
// in the given direction. The two back corners sit 10 units behind
// the tip along the direction of travel, offset +/-6 units
// perpendicular to it (the direction vector rotated 90 degrees),
// giving a narrow isoceles triangle.
SEAM_EDITOR.drawArrow = function(c, x, y, angle) {
	var directionX = Math.cos(angle);
	var directionY = Math.sin(angle);
	c.beginPath();
	c.moveTo(x, y);
	c.lineTo(x - 10 * directionX + 6 * directionY, y - 10 * directionY - 6 * directionX);
	c.lineTo(x - 10 * directionX - 6 * directionY, y - 10 * directionY + 6 * directionX);
	c.fill();
}

SEAM_EDITOR.canvasHasFocus = function() {
	return (document.activeElement || document.body) == document.body;
}

// Caret helpers for multi-line node labels
SEAM_EDITOR.caretToLineCol = function(text, pos) {
	var lines = text.split('\n');
	var remaining = pos;
	for(var i = 0; i < lines.length; i++) {
		if(remaining <= lines[i].length) return { line: i, col: remaining };
		remaining -= lines[i].length + 1;
	}
	var last = lines.length - 1;
	return { line: last, col: lines[last].length };
}

SEAM_EDITOR.lineColToPos = function(text, targetLine, targetCol) {
	var lines = text.split('\n');
	var line = Math.min(targetLine, lines.length - 1);
	var pos = 0;
	for(var i = 0; i < line; i++) pos += lines[i].length + 1;
	return pos + Math.min(targetCol, lines[line].length);
}

// Multi-line rendering for node labels (angleOrNull === null).
// cx/cy is the centre of the node.
SEAM_EDITOR._drawTextMultiline = function(c, text, originalText, cx, cy, isSelected) {
	var lines = text.split('\n');
	var maxWidth = 0;
	for(var i = 0; i < lines.length; i++) maxWidth = Math.max(maxWidth, c.measureText(lines[i]).width);
	var totalHeight = lines.length * SEAM_EDITOR.seamLineHeight;

	if('advancedFillText' in c) {
		// LaTeX export: render each line with y offset
		var startYex = cy - (lines.length - 1) * SEAM_EDITOR.seamLineHeight / 2;
		for(var i = 0; i < lines.length; i++)
			c.advancedFillText(lines[i], lines[i], cx, startYex + i * SEAM_EDITOR.seamLineHeight, null);
		return;
	}

	cx = Math.round(cx); cy = Math.round(cy);
	var editing = isSelected && SEAM_EDITOR.editingLabel && SEAM_EDITOR.canvasHasFocus() && document.hasFocus();
	if(editing && SEAM_EDITOR._suppressEditBox) return; // deferred to top-most overlay pass

	if(editing) {
		var pad = 7;
		var bw = Math.max(maxWidth, 20) + pad * 2;
		var bh = totalHeight + pad * 2;
		var bx = cx - Math.max(maxWidth, 20) / 2 - pad;
		var by = cy - totalHeight / 2 - pad;
		var sf = c.fillStyle, ss = c.strokeStyle, slw = c.lineWidth;
		c.fillStyle = SEAM_EDITOR.seamEditBg; c.strokeStyle = SEAM_EDITOR.seamEditBorder; c.lineWidth = 1;
		c.beginPath();
		if(c.roundRect) c.roundRect(bx, by, bw, bh, 4); else c.rect(bx, by, bw, bh);
		c.fill(); c.stroke();
		c.fillStyle = sf; c.strokeStyle = ss; c.lineWidth = slw;
	}

	var savedFill = c.fillStyle;
	c.fillStyle = SEAM_EDITOR.seamActiveLabelColor || savedFill;
	var startY = cy - (lines.length - 1) * SEAM_EDITOR.seamLineHeight / 2;
	for(var i = 0; i < lines.length; i++) {
		var lw = c.measureText(lines[i]).width;
		c.fillText(lines[i], cx - lw / 2, startY + i * SEAM_EDITOR.seamLineHeight + 5);
	}
	c.fillStyle = savedFill;

	if(editing && SEAM_EDITOR.caretVisible) {
		var lc = SEAM_EDITOR.caretToLineCol(originalText, Math.min(SEAM_EDITOR.caretPosition, originalText.length));
		var cli = Math.min(lc.line, lines.length - 1);
		var clt = lines[cli] || '';
		var col = Math.min(lc.col, clt.length);
		var caretX = cx - c.measureText(clt).width / 2 + c.measureText(clt.substring(0, col)).width;
		var caretY = startY + cli * SEAM_EDITOR.seamLineHeight;
		c.beginPath();
		c.moveTo(caretX, caretY - 7); c.lineTo(caretX, caretY + 9);
		c.stroke();
	}
}

SEAM_EDITOR.drawText = function(c, originalText, x, y, angleOrNull, isSelected) {
	var rawText = SEAM_EDITOR.convertLatexShortcuts(originalText);
	c.font = SEAM_EDITOR.seamFont;

	// Node labels: delegate to multi-line renderer (no angle offset)
	if(angleOrNull === null) {
		SEAM_EDITOR._drawTextMultiline(c, rawText, originalText, x, y, isSelected);
		return;
	}

	// Link labels: multi-line, with the lineAngleAdjust offset applied for block height
	var lines = rawText.split('\n');
	var maxWidth = 0;
	for(var i = 0; i < lines.length; i++) maxWidth = Math.max(maxWidth, c.measureText(lines[i]).width);
	var totalHeight = lines.length * SEAM_EDITOR.seamLineHeight;

	var cos = Math.cos(angleOrNull);
	var sin = Math.sin(angleOrNull);
	// Both overrides are continuous ±1 floats animated by Link.draw.
	// Used as direct multipliers so both axes tween smoothly.
	var sinForSide = (SEAM_EDITOR._drawTextSinOverride !== null) ? SEAM_EDITOR._drawTextSinOverride : (sin >= 0 ? 1 : -1);
	var cosForSide = (SEAM_EDITOR._drawTextCosOverride !== null) ? SEAM_EDITOR._drawTextCosOverride : (cos >= 0 ? 1 : -1);
	var cornerPointX = (maxWidth / 2 + 5) * cosForSide;  // continuous, not sign-only
	var cornerPointY = (totalHeight / 2 + 5) * sinForSide; // continuous, not sign-only
	var slide = sin * Math.pow(Math.abs(sin), 40) * cornerPointX - cos * Math.pow(Math.abs(cos), 10) * cornerPointY;
	// cx/cy: centre of the text block in canvas space
	var cx = x + cornerPointX - sin * slide;
	var cy = y + cornerPointY + cos * slide;

	if('advancedFillText' in c) {
		// Export renderers: emit each line
		for(var i = 0; i < lines.length; i++) {
			var lineY = cy - (lines.length - 1) * SEAM_EDITOR.seamLineHeight / 2 + i * SEAM_EDITOR.seamLineHeight;
			c.advancedFillText(lines[i], lines[i], cx, lineY, angleOrNull);
		}
	} else {
		cx = Math.round(cx); cy = Math.round(cy);
		var editing = isSelected && SEAM_EDITOR.editingLabel && SEAM_EDITOR.canvasHasFocus() && document.hasFocus();

		if(editing) {
			// Axis-aligned edit rect (no rotation — labels always face the user)
			var pad = 7;
			var bw = Math.max(maxWidth, 20) + pad * 2;
			var bh = totalHeight + pad * 2;
			var sf = c.fillStyle, ss = c.strokeStyle, slw = c.lineWidth;
			c.fillStyle = SEAM_EDITOR.seamEditBg; c.strokeStyle = SEAM_EDITOR.seamEditBorder; c.lineWidth = 1;
			c.beginPath();
			if(c.roundRect) c.roundRect(cx - bw / 2, cy - bh / 2, bw, bh, 4);
			else c.rect(cx - bw / 2, cy - bh / 2, bw, bh);
			c.fill(); c.stroke();
			c.fillStyle = sf; c.strokeStyle = ss; c.lineWidth = slw;
		}

		// Draw each line centred around cx
		var savedFill = c.fillStyle;
		c.fillStyle = SEAM_EDITOR.seamActiveLabelColor || savedFill;
		var startY = cy - (lines.length - 1) * SEAM_EDITOR.seamLineHeight / 2;
		for(var i = 0; i < lines.length; i++) {
			var lw = c.measureText(lines[i]).width;
			c.fillText(lines[i], cx - lw / 2, startY + i * SEAM_EDITOR.seamLineHeight + 5);
		}
		c.fillStyle = savedFill;

		// Caret — drawn in rotated coordinate frame
		if(editing && SEAM_EDITOR.caretVisible) {
			var lc = SEAM_EDITOR.caretToLineCol(originalText, Math.min(SEAM_EDITOR.caretPosition, originalText.length));
			var cli = Math.min(lc.line, lines.length - 1);
			var clt = lines[cli] || '';
			var col = Math.min(lc.col, clt.length);
			// Local x in the line (relative to line centre)
			var localX = -c.measureText(clt).width / 2 + c.measureText(clt.substring(0, col)).width;
			var localY = startY + cli * SEAM_EDITOR.seamLineHeight - cy; // relative to cy
			// Map local to canvas via rotation
			var caretCanvasX = cx + localX * Math.cos(0) - localY * Math.sin(0); // angle=0: text isn't actually rotated
			var caretCanvasY = cy + localY;
			c.beginPath();
			c.moveTo(cx + localX, caretCanvasY - 7);
			c.lineTo(cx + localX, caretCanvasY + 9);
			c.stroke();
		}
	}
}

SEAM_EDITOR.caretTimer = undefined;
SEAM_EDITOR.caretVisible = true;
SEAM_EDITOR.caretPosition = 0;  // index into selectedObject.text

SEAM_EDITOR.resetCaret = function() {
	clearInterval(SEAM_EDITOR.caretTimer);
	SEAM_EDITOR.caretTimer = setInterval('SEAM_EDITOR.caretVisible = !SEAM_EDITOR.caretVisible; SEAM_EDITOR.draw()', 500);
	SEAM_EDITOR.caretVisible = true;
}

// Edit-mode label background — set by Lit shell _applyTheme
SEAM_EDITOR.seamEditBg     = 'rgba(248,250,252,0.95)';
SEAM_EDITOR.seamEditBorder = 'rgba(99,102,241,0.5)';
// Temporary-link animation
SEAM_EDITOR._linkPulseRafId  = null;  // RAF id for pulsating opacity during drag
SEAM_EDITOR._linkPulseT      = 0;     // time accumulator (ms)
SEAM_EDITOR._linkPulseAlpha  = 1;     // current globalAlpha for TemporaryLink draw
SEAM_EDITOR._linkPulseLast   = null;  // DOMHighResTimeStamp of last frame
SEAM_EDITOR._snapBackRafId   = null;  // RAF id for snap-back animation
SEAM_EDITOR._snapBackLink    = null;  // frozen TemporaryLink during snap-back
SEAM_EDITOR._snapBackActive  = false; // true while snap-back is running

SEAM_EDITOR._startLinkPulse = function() {
	if(SEAM_EDITOR._linkPulseRafId !== null) return;
	SEAM_EDITOR._linkPulseLast = null;
	function pulseFrame(now) {
		if(SEAM_EDITOR._linkPulseLast === null) SEAM_EDITOR._linkPulseLast = now;
		SEAM_EDITOR._linkPulseT += now - SEAM_EDITOR._linkPulseLast;
		SEAM_EDITOR._linkPulseLast = now;
		var sine = Math.sin(SEAM_EDITOR._linkPulseT * 0.006); // ~1 Hz
		SEAM_EDITOR._linkPulseAlpha = 0.35 + 0.65 * (sine * 0.5 + 0.5); // range 0.35..1
		if(SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.TemporaryLink && !SEAM_EDITOR._snapBackActive) {
			SEAM_EDITOR.draw();
			SEAM_EDITOR._linkPulseRafId = requestAnimationFrame(pulseFrame);
		} else {
			SEAM_EDITOR._linkPulseAlpha = 1;
			SEAM_EDITOR._linkPulseRafId = null;
		}
	}
	SEAM_EDITOR._linkPulseRafId = requestAnimationFrame(pulseFrame);
}

SEAM_EDITOR._stopLinkPulse = function() {
	if(SEAM_EDITOR._linkPulseRafId !== null) {
		cancelAnimationFrame(SEAM_EDITOR._linkPulseRafId);
		SEAM_EDITOR._linkPulseRafId = null;
	}
	SEAM_EDITOR._linkPulseAlpha = 1;
	SEAM_EDITOR._linkPulseLast  = null;
}

SEAM_EDITOR._startSnapBack = function(link) {
	SEAM_EDITOR._stopLinkPulse();
	SEAM_EDITOR._snapBackLink   = { from: { x: link.from.x, y: link.from.y },
	                    to:   { x: link.to.x,   y: link.to.y   } };
	SEAM_EDITOR._snapBackActive = true;
	SEAM_EDITOR.currentLink     = null;
	var startX = SEAM_EDITOR._snapBackLink.to.x, startY = SEAM_EDITOR._snapBackLink.to.y;
	var endX   = SEAM_EDITOR._snapBackLink.from.x, endY = SEAM_EDITOR._snapBackLink.from.y;
	var startT = performance.now();
	var dur    = 520; // ms
	function snapFrame(now) {
		var t = Math.min((now - startT) / dur, 1);
		var ease = 1 - Math.pow(1 - t, 3); // ease-out cubic
		SEAM_EDITOR._snapBackLink.to.x = startX + (endX - startX) * ease;
		SEAM_EDITOR._snapBackLink.to.y = startY + (endY - startY) * ease;
		var alpha = 1 - ease * 0.85; // fades toward 0.15 then gone
		SEAM_EDITOR._linkPulseAlpha = alpha;
		SEAM_EDITOR.draw();
		if(t < 1) {
			SEAM_EDITOR._snapBackRafId = requestAnimationFrame(snapFrame);
		} else {
			SEAM_EDITOR._snapBackActive = false;
			SEAM_EDITOR._snapBackLink   = null;
			SEAM_EDITOR._linkPulseAlpha = 1;
			SEAM_EDITOR._snapBackRafId  = null;
			SEAM_EDITOR.draw();
		}
	}
	SEAM_EDITOR._snapBackRafId = requestAnimationFrame(snapFrame);
}

// Label editing: only active after stationary mouseup
SEAM_EDITOR.editingLabel       = false;
SEAM_EDITOR._suppressEditBox   = false; // true during main pass; edit overlay drawn last
SEAM_EDITOR._hasDragged    = false;
// Halo: node primed on exit; opacity animates in fast, out slow
SEAM_EDITOR._haloNode      = null;
// Link hover: null or the link under the mouse pointer
SEAM_EDITOR._hoveredLink   = null;
// Dot: separate wider-tolerance hover for midpoint dot animation
SEAM_EDITOR._dotLink       = null;
SEAM_EDITOR._dotOpacity    = 0;
SEAM_EDITOR._dotTarget     = 0;
SEAM_EDITOR._dotRafId      = null;

SEAM_EDITOR._startDotAnim = function() {
	if(SEAM_EDITOR._dotRafId !== null) return;
	function dotFrame() {
		var step = (SEAM_EDITOR._dotTarget > SEAM_EDITOR._dotOpacity) ? 0.15 : 0.09;
		var diff = SEAM_EDITOR._dotTarget - SEAM_EDITOR._dotOpacity;
		if(Math.abs(diff) < 0.004) {
			SEAM_EDITOR._dotOpacity = SEAM_EDITOR._dotTarget;
			SEAM_EDITOR._dotRafId = null;
			if(SEAM_EDITOR._dotTarget === 0) SEAM_EDITOR._dotLink = null;
			SEAM_EDITOR.draw();
			return;
		}
		SEAM_EDITOR._dotOpacity += (diff > 0 ? 1 : -1) * Math.min(step, Math.abs(diff));
		SEAM_EDITOR.draw();
		SEAM_EDITOR._dotRafId = requestAnimationFrame(dotFrame);
	}
	SEAM_EDITOR._dotRafId = requestAnimationFrame(dotFrame);
}
SEAM_EDITOR._haloVisible   = false;  // logical state (target)
SEAM_EDITOR._haloOpacity   = 0;      // current rendered opacity 0..1
SEAM_EDITOR._haloTarget    = 0;      // 0 or 1
SEAM_EDITOR._haloRafId     = null;

SEAM_EDITOR._startNodeGrow = function(node) {
	SEAM_EDITOR._growingNode = node;
	SEAM_EDITOR._growT = 0;
	if(SEAM_EDITOR._growRafId !== null) return;
	var lastT = null;
	function growFrame(t) {
		if(lastT === null) lastT = t;
		var dt = Math.min(t - lastT, 64); lastT = t;
		SEAM_EDITOR._growT += dt;
		if(SEAM_EDITOR._growT >= SEAM_EDITOR._growDur) {
			SEAM_EDITOR._growT = SEAM_EDITOR._growDur;
			SEAM_EDITOR._growingNode = null;
			SEAM_EDITOR._growRafId = null;
			SEAM_EDITOR.draw(); return;
		}
		SEAM_EDITOR.draw();
		SEAM_EDITOR._growRafId = requestAnimationFrame(growFrame);
	}
	SEAM_EDITOR._growRafId = requestAnimationFrame(growFrame);
}

SEAM_EDITOR._startGroupHalo = function() {
	if(SEAM_EDITOR._groupHaloRafId !== null) return;
	var lastT = null;
	function groupHaloFrame(t) {
		if(SEAM_EDITOR._selectedNodes.length === 0) { SEAM_EDITOR._groupHaloRafId = null; return; }
		if(lastT === null) lastT = t;
		var dt = Math.min(t - lastT, 64); lastT = t;
		SEAM_EDITOR._groupHaloT += dt;
		SEAM_EDITOR._groupHaloAlpha = 0.45 + 0.45 * Math.sin(SEAM_EDITOR._groupHaloT * 0.007);
		SEAM_EDITOR.draw();
		SEAM_EDITOR._groupHaloRafId = requestAnimationFrame(groupHaloFrame);
	}
	SEAM_EDITOR._groupHaloRafId = requestAnimationFrame(groupHaloFrame);
}

SEAM_EDITOR._startHaloAnim = function() {
	if(SEAM_EDITOR._haloRafId !== null) return;
	function haloFrame() {
		var step = (SEAM_EDITOR._haloTarget > SEAM_EDITOR._haloOpacity) ? 0.18 : 0.07; // fast in, slow out
		var diff = SEAM_EDITOR._haloTarget - SEAM_EDITOR._haloOpacity;
		if(Math.abs(diff) < 0.005) {
			SEAM_EDITOR._haloOpacity = SEAM_EDITOR._haloTarget;
			SEAM_EDITOR._haloRafId = null;
			if(SEAM_EDITOR._haloTarget === 0) SEAM_EDITOR._haloNode = null; // fully faded — release node ref
			SEAM_EDITOR.draw();
			return;
		}
		SEAM_EDITOR._haloOpacity += (diff > 0 ? 1 : -1) * Math.min(step, Math.abs(diff));
		SEAM_EDITOR.draw();
		SEAM_EDITOR._haloRafId = requestAnimationFrame(haloFrame);
	}
	SEAM_EDITOR._haloRafId = requestAnimationFrame(haloFrame);
}
// Per-link guard/action properties (keyed by link._id)
SEAM_EDITOR._linkIdCounter = 0;
SEAM_EDITOR.linkProperties = new Map(); // _id → {guard, action}
// Per-link custom colour. Cycled by Shift+click.
SEAM_EDITOR.linkColors = new Map(); // _id → colour string

SEAM_EDITOR.canvas = undefined;
SEAM_EDITOR.nodeRadius = 30;
SEAM_EDITOR.nodes = [];
SEAM_EDITOR.links = [];

SEAM_EDITOR.cursorVisible = true;
SEAM_EDITOR.snapToPadding = 6; // pixels
SEAM_EDITOR.hitTargetPadding = 6; // pixels
SEAM_EDITOR.selectedObject = null; // either a Link or a Node
SEAM_EDITOR.currentLink = null; // a Link
SEAM_EDITOR.movingObject = false;
SEAM_EDITOR.originalClick = undefined;

// ── Viewport (pan + zoom) ─────────────────────────────────────────────────────
SEAM_EDITOR.viewport = { x: 0, y: 0, k: 1 };
// Font for all labels — writable by the Lit shell to sync with theme
SEAM_EDITOR.seamFont     = '12px system-ui, -apple-system, "Segoe UI", sans-serif';
SEAM_EDITOR.seamLineHeight = 16;  // px between baselines for multi-line labels
// Visual style globals — set by Lit shell _applyTheme
SEAM_EDITOR.seamAutoSize       = false;        // adapt node radius to label width
SEAM_EDITOR.seamAutoSizePad    = 14;           // inner padding (px) for auto-sized nodes
// Link label side-flip smoothing: set by Link.draw, read by drawText
SEAM_EDITOR._drawTextSinOverride = null; // float set by Link.draw, read by drawText
SEAM_EDITOR._drawTextCosOverride = null; // float set by Link.draw, read by drawText
SEAM_EDITOR._labelFlipDur = 450;         // ms for one full side transition

SEAM_EDITOR._smoothLabelAxis = function(link, targetVal, sideKey, fromKey, toKey, t0Key) {
	var target = targetVal >= 0 ? 1 : -1;
	if(link[sideKey] === undefined || link[sideKey] === null) {
		link[sideKey] = target; link[toKey] = target;
		link[fromKey] = target; link[t0Key]  = null;
		return target;
	}
	if(target !== link[toKey]) {
		link[fromKey] = link[sideKey];
		link[toKey]   = target;
		link[t0Key]   = performance.now();
		SEAM_EDITOR._scheduleLabelRaf();
	}
	return link[sideKey];
}

SEAM_EDITOR._smoothLabelSide = function(link, targetSin) {
	return SEAM_EDITOR._smoothLabelAxis(link, targetSin, '_labelSide', '_labelFrom', '_labelTo', '_labelT0');
}

SEAM_EDITOR._smoothLabelCos = function(link, targetCos) {
	return SEAM_EDITOR._smoothLabelAxis(link, targetCos, '_labelCosS', '_labelCosFrom', '_labelCosTo', '_labelCosT0');
}

SEAM_EDITOR._labelFlipRafId = null;

SEAM_EDITOR._scheduleLabelRaf = function() {
	if(SEAM_EDITOR._labelFlipRafId !== null) return;
	SEAM_EDITOR._labelFlipRafId = requestAnimationFrame(SEAM_EDITOR._labelFlipFrame);
}

SEAM_EDITOR._labelFlipFrame = function(now) {
	SEAM_EDITOR._labelFlipRafId = null;
	var stillAnimating = false;
	for(var i = 0; i < SEAM_EDITOR.links.length; i++) {
		var lnk = SEAM_EDITOR.links[i];
		// Sin axis
		if(lnk._labelT0) {
			var t = Math.min((now - lnk._labelT0) / SEAM_EDITOR._labelFlipDur, 1);
			var ease = t * t * (3 - 2 * t);
			lnk._labelSide = lnk._labelFrom + (lnk._labelTo - lnk._labelFrom) * ease;
			if(t < 1) { stillAnimating = true; }
			else { lnk._labelSide = lnk._labelTo; lnk._labelT0 = null; }
		}
		// Cos axis
		if(lnk._labelCosT0) {
			var tc = Math.min((now - lnk._labelCosT0) / SEAM_EDITOR._labelFlipDur, 1);
			var easec = tc * tc * (3 - 2 * tc);
			lnk._labelCosS = lnk._labelCosFrom + (lnk._labelCosTo - lnk._labelCosFrom) * easec;
			if(tc < 1) { stillAnimating = true; }
			else { lnk._labelCosS = lnk._labelCosTo; lnk._labelCosT0 = null; }
		}
	}
	SEAM_EDITOR.draw();
	if(stillAnimating) SEAM_EDITOR._labelFlipRafId = requestAnimationFrame(SEAM_EDITOR._labelFlipFrame);
}
SEAM_EDITOR.seamNodeFill       = '#f8fafc';   // node interior fill
SEAM_EDITOR.seamAccentColor    = '#169B62';   // Irish green for accept states
SEAM_EDITOR.seamNodeStroke     = 1.5;         // circle stroke width
SEAM_EDITOR.seamLinkStroke     = 2.0;         // arc/line stroke width
SEAM_EDITOR.seamNodeStrokeColor = '#6366f1';  // node circle colour
SEAM_EDITOR.seamNodeLabelColor  = '#1e293b';  // text inside nodes
SEAM_EDITOR.seamLinkStrokeColor = '#818cf8';  // transition arc colour
SEAM_EDITOR.seamLinkLabelColor  = '#64748b';  // text on transitions
SEAM_EDITOR.seamActiveLabelColor = '#1e293b'; // set by drawUsing before each drawText
// Zoom inertia
SEAM_EDITOR._zoomVelocity = 0;
SEAM_EDITOR._zoomCursorX = 0;
SEAM_EDITOR._zoomCursorY = 0;
SEAM_EDITOR._zoomRafId   = null;
SEAM_EDITOR.panningCanvas = false;
SEAM_EDITOR.panStart = null;
// Add-linked-node mode (right-click halo)
SEAM_EDITOR._addingLinkedNode = false;  // true when drag is in add-linked-node mode
SEAM_EDITOR._addLinkedOrigin  = null;   // the source node for the new link
// Node grow animation
SEAM_EDITOR._growingNode  = null;  // Node currently animating in
SEAM_EDITOR._growT        = 0;     // elapsed ms
SEAM_EDITOR._growDur      = 320;   // ms for full grow
SEAM_EDITOR._growRafId    = null;

// Group (rubber-band) selection
SEAM_EDITOR._selectedNodes  = [];     // nodes in the current group selection
SEAM_EDITOR._rectSelecting  = false;  // true while right-drag rubber band is active
SEAM_EDITOR._rectSelStart   = null;   // world coords of rubber-band anchor
SEAM_EDITOR._rectSelCur     = null;   // world coords of current mouse pos
// Group-halo pulse (continuous sin wave, same pattern as link pulse)
SEAM_EDITOR._groupHaloAlpha = 0.55;
SEAM_EDITOR._groupHaloRafId = null;
SEAM_EDITOR._groupHaloT     = 0;
SEAM_EDITOR.panViewStart = null;

SEAM_EDITOR.clearCanvas = function() {
	SEAM_EDITOR.stopForceLayout(); // stop any running simulation before the nodes it references disappear
	SEAM_EDITOR.nodes = [];
	SEAM_EDITOR.links = [];
	SEAM_EDITOR.linkProperties.clear();
	SEAM_EDITOR.linkColors.clear();
	SEAM_EDITOR._selectedNodes = [];
	SEAM_EDITOR._rectSelecting = false;
	SEAM_EDITOR._groupHaloRafId = null;
	SEAM_EDITOR._addingLinkedNode = false;
	SEAM_EDITOR._addLinkedOrigin  = null;
	SEAM_EDITOR._growingNode = null;
	localStorage.removeItem('fsm');
	SEAM_EDITOR.viewport = { x: 0, y: 0, k: 1 };
	var context = SEAM_EDITOR.canvas.getContext('2d');
	context.clearRect(0, 0, SEAM_EDITOR.canvas.width, SEAM_EDITOR.canvas.height);
	SEAM_EDITOR.nodeRadius = 30;
}

// ============================================================
// Force-directed layout.
//
// Added to this canvas engine so it can also drive
// automatically-laid-out graphs (e.g. an asset-relationship view),
// not only hand-placed FSM diagrams. This is Seam's own extension,
// same as the drag/selection/label/zoom features elsewhere in this
// file.
//
// The physics itself is adapted from d3-force
// (https://github.com/d3/d3-force), Copyright 2010-2021 Mike
// Bostock, ISC License:
//   Permission to use, copy, modify, and/or distribute this software
//   for any purpose with or without fee is hereby granted, provided
//   that the above copyright notice and this permission notice
//   appear in all copies.
//
// Deliberately simplified from the original in one significant way:
// the many-body repulsion below is a direct O(n^2) pairwise sum, not
// d3-force's own Barnes-Hut quadtree approximation (its
// manyBody.js and collide.js both use one). Same physics, same
// result -- just without the O(n
// log n) speedup, which isn't needed at the node counts this engine
// actually renders (a single facility's assets, not force layouts
// with thousands of nodes). If that stops being true, port the
// quadtree approximation from the same source rather than tuning
// this away -- the accuracy trade-off is deliberate now, not yet a
// proven bottleneck.
SEAM_EDITOR._forceRafId = null;
SEAM_EDITOR._forceAlpha = 0;
SEAM_EDITOR._forceAlphaMin = 0.001;
SEAM_EDITOR._forceAlphaDecay = 1 - Math.pow(SEAM_EDITOR._forceAlphaMin, 1 / 300); // same ~300-tick cooling curve as d3-force
SEAM_EDITOR._forceAlphaTarget = 0;
SEAM_EDITOR._forceVelocityDecay = 0.6; // fraction of velocity KEPT each tick (0.6 = 40% "friction")
SEAM_EDITOR._forceLinkDistance = 120;
SEAM_EDITOR._forceChargeStrength = -20; // negative = repulsion
// Correction (found live, testing against real seed data --
// seeds/seed-01-constructec.py -- rather than a small synthetic
// graph): -300 was an arbitrary choice never validated against a
// realistic node count. At 26 real nodes (16 assets + 10 sensors),
// -300 pushed most of the layout hundreds of pixels past any
// reasonable viewport -- stable (no oscillation), but essentially
// unusable, since the vast majority of nodes settled off-screen. This
// only showed up once a real dataset's actual scale was tested; the
// original 10-node synthetic example never exercised enough
// cumulative repulsion (each node only feels every OTHER node's
// push, so total force per node scales with node count) to reveal it.
// -20 -- close to d3-force's own real, widely-used default of -30 --
// was chosen empirically: swept several values against the real
// 26-node graph across 40 random starting layouts each, checking both
// final overlap (must stay at zero) and how many nodes end up outside
// a reasonable viewport. -20 was the best of the values tried on both
// counts simultaneously.

SEAM_EDITOR._forceJiggle = function() {
	// Tiny random nudge so two exactly-coincident nodes don't produce a
	// zero-length vector, which would divide by zero in the repulsion/
	// spring math below. Same purpose as d3-force's own jiggle.js.
	return (Math.random() - 0.5) * 1e-6;
}

SEAM_EDITOR._forceManyBody = function(alpha) {
	// O(n^2) pairwise repulsion -- see the file-level comment above on
	// why this isn't the Barnes-Hut approximation d3-force itself uses.
	//
	// Correction (found live, after shipping): the a/b sign assignment
	// below was originally backwards (a.vx -=, b.vx +=), which produces
	// ATTRACTION for a negative charge strength, not repulsion --
	// exactly opposite of the intent. d3-force's own manyBody.js applies
	// a single node's velocity update as `node.vx += x * w` where x
	// points from that node toward the other one; the fix here mirrors
	// that per node: a moves along +dx*w (away from b, since w is
	// negative), b moves along -dx*w (away from a). Verified directly:
	// two nodes at a reasonable, non-degenerate separation now move
	// apart, not together -- the original bug's own "passing" test only
	// used two nearly-coincident starting points, where a numerical
	// artifact from the distanceMin floor coincidentally produced
	// outward-looking motion and masked the sign error completely.
	// This single bug was the actual cause of nodes clustering instead
	// of spreading out, and of the sustained flicker reported once
	// _forceCollide was added -- collide correctly pushing overlapping
	// nodes apart was fighting against many-body incorrectly pulling
	// them back together, every tick.
	for (var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		var a = SEAM_EDITOR.nodes[i];
		for (var j = i + 1; j < SEAM_EDITOR.nodes.length; j++) {
			var b = SEAM_EDITOR.nodes[j];
			var dx = b.x - a.x;
			var dy = b.y - a.y;
			if (dx === 0) dx = SEAM_EDITOR._forceJiggle();
			if (dy === 0) dy = SEAM_EDITOR._forceJiggle();
			var l = dx * dx + dy * dy;
			if (l < 1) l = 1; // distanceMin -- avoid a runaway force at near-zero distance
			var w = (SEAM_EDITOR._forceChargeStrength * alpha) / l;
			a.vx += dx * w;
			a.vy += dy * w;
			b.vx -= dx * w;
			b.vy -= dy * w;
		}
	}
}

SEAM_EDITOR._forceLinkSpring = function(alpha) {
	// Pulls each link's two nodes toward _forceLinkDistance apart. The
	// "bias" weighting matches d3-force's own link.js: a node with many
	// links moves less per single correction than one with few, so a
	// busy hub node doesn't get yanked around by every edge touching it.
	var count = [];
	var i, link, a, b, ai, bi;
	for (i = 0; i < SEAM_EDITOR.links.length; i++) {
		link = SEAM_EDITOR.links[i];
		if (!(link.nodeA instanceof SEAM_EDITOR.Node) || !(link.nodeB instanceof SEAM_EDITOR.Node)) continue; // self-links/temp links during drawing
		ai = SEAM_EDITOR.nodes.indexOf(link.nodeA);
		bi = SEAM_EDITOR.nodes.indexOf(link.nodeB);
		count[ai] = (count[ai] || 0) + 1;
		count[bi] = (count[bi] || 0) + 1;
	}
	for (i = 0; i < SEAM_EDITOR.links.length; i++) {
		link = SEAM_EDITOR.links[i];
		if (!(link.nodeA instanceof SEAM_EDITOR.Node) || !(link.nodeB instanceof SEAM_EDITOR.Node) || link.nodeA === link.nodeB) continue;
		a = link.nodeA;
		b = link.nodeB;
		ai = SEAM_EDITOR.nodes.indexOf(a);
		bi = SEAM_EDITOR.nodes.indexOf(b);
		var dx = (b.x + b.vx) - (a.x + a.vx);
		var dy = (b.y + b.vy) - (a.y + a.vy);
		if (dx === 0) dx = SEAM_EDITOR._forceJiggle();
		if (dy === 0) dy = SEAM_EDITOR._forceJiggle();
		var l = Math.sqrt(dx * dx + dy * dy);
		l = (l - SEAM_EDITOR._forceLinkDistance) / l * alpha;
		dx *= l; dy *= l;
		var bias = count[bi] / (count[ai] + count[bi]);
		b.vx -= dx * bias;
		b.vy -= dy * bias;
		a.vx += dx * (1 - bias);
		a.vy += dy * (1 - bias);
	}
}

SEAM_EDITOR._forceCenter = function(cx, cy) {
	// Recenters the whole layout each tick without changing any node's
	// position relative to the others -- keeps the graph drifting
	// toward the canvas middle instead of wandering off-screen.
	if (SEAM_EDITOR.nodes.length === 0) return;
	var sx = 0, sy = 0, i;
	for (i = 0; i < SEAM_EDITOR.nodes.length; i++) { sx += SEAM_EDITOR.nodes[i].x; sy += SEAM_EDITOR.nodes[i].y; }
	sx = sx / SEAM_EDITOR.nodes.length - cx;
	sy = sy / SEAM_EDITOR.nodes.length - cy;
	for (i = 0; i < SEAM_EDITOR.nodes.length; i++) { SEAM_EDITOR.nodes[i].x -= sx; SEAM_EDITOR.nodes[i].y -= sy; }
}

// Added after initial testing surfaced real overlap between differently-
// sized nodes (a small node and a large one settling closer together
// than their radii allow) -- _forceManyBody's point-charge repulsion has
// no concept of a node's own radius, so it cannot by itself guarantee two
// nodes never end up closer than the sum of their sizes. This is a
// separate, hard minimum-distance constraint, adapted from d3-force's own
// collide.js (same source/license as the other forces above): if two
// nodes are closer than (radiusA + radiusB), push them apart along the
// line between them by the overlap amount, weighted so the smaller node
// moves more than the larger one (matching physical intuition -- a small
// node gets shoved aside more readily than a large one).
//
// Verified against the exact scenario that surfaced the gap: a large
// node and a small node started nearly coincident, and a cluster of five
// small nodes started overlapping one large one -- both converge to zero
// overlap. Also re-verified with all four forces running together
// (charge + link + collide + center) on the actual asset-relationship
// graph shape (one hub, several mid-sized assets, several small sensors)
// -- settles cleanly with no pairwise overlap anywhere in the graph.
SEAM_EDITOR._forceCollide = function() {
	for (var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		var a = SEAM_EDITOR.nodes[i];
		var ri = a._r != null ? a._r : SEAM_EDITOR.nodeRadius;
		for (var j = i + 1; j < SEAM_EDITOR.nodes.length; j++) {
			var b = SEAM_EDITOR.nodes[j];
			var rj = b._r != null ? b._r : SEAM_EDITOR.nodeRadius;
			var r = ri + rj;
			var dx = (b.x + b.vx) - (a.x + a.vx);
			var dy = (b.y + b.vy) - (a.y + a.vy);
			var l = dx * dx + dy * dy;
			if (l < r * r) {
				if (dx === 0) { dx = SEAM_EDITOR._forceJiggle(); l += dx * dx; }
				if (dy === 0) { dy = SEAM_EDITOR._forceJiggle(); l += dy * dy; }
				l = Math.sqrt(l);
				var overlap = (r - l) / l;
				dx *= overlap; dy *= overlap;
				var share = (rj * rj) / (ri * ri + rj * rj);
				a.vx -= dx * share;
				a.vy -= dy * share;
				b.vx += dx * (1 - share);
				b.vy += dy * (1 - share);
			}
		}
	}
}

SEAM_EDITOR._forceTick = function() {
	SEAM_EDITOR._forceAlpha += (SEAM_EDITOR._forceAlphaTarget - SEAM_EDITOR._forceAlpha) * SEAM_EDITOR._forceAlphaDecay;
	SEAM_EDITOR._forceManyBody(SEAM_EDITOR._forceAlpha);
	SEAM_EDITOR._forceLinkSpring(SEAM_EDITOR._forceAlpha);
	SEAM_EDITOR._forceCollide();
	for (var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		var node = SEAM_EDITOR.nodes[i];
		if (node.fx == null) { node.x += node.vx *= SEAM_EDITOR._forceVelocityDecay; }
		else { node.x = node.fx; node.vx = 0; }
		if (node.fy == null) { node.y += node.vy *= SEAM_EDITOR._forceVelocityDecay; }
		else { node.y = node.fy; node.vy = 0; }
	}
	if (SEAM_EDITOR.canvas) SEAM_EDITOR._forceCenter(SEAM_EDITOR.canvas.width / 2, SEAM_EDITOR.canvas.height / 2);
}

/**
 * Starts (or restarts, e.g. after a drag) the force-directed layout
 * simulation. Exposed as window.startForceLayout so a consumer other
 * than the FSM editor's own Lit shell -- an asset-relationship graph
 * view, for instance -- can trigger automatic layout on this same
 * canvas engine. Runs at one tick per animation frame, same cadence
 * d3-force itself uses (no delta-time scaling -- the physics is
 * defined in "ticks", not seconds, exactly as in the source this was
 * adapted from).
 */
SEAM_EDITOR.startForceLayout = function(options) {
	options = options || {};
	if (options.linkDistance != null) SEAM_EDITOR._forceLinkDistance = options.linkDistance;
	if (options.chargeStrength != null) SEAM_EDITOR._forceChargeStrength = options.chargeStrength;
	for (var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		if (SEAM_EDITOR.nodes[i].vx == null) SEAM_EDITOR.nodes[i].vx = 0;
		if (SEAM_EDITOR.nodes[i].vy == null) SEAM_EDITOR.nodes[i].vy = 0;
	}
	SEAM_EDITOR._forceAlpha = 1;
	if (SEAM_EDITOR._forceRafId === null) {
		function forceFrame() {
			SEAM_EDITOR._forceTick();
			SEAM_EDITOR.draw();
			if (SEAM_EDITOR._forceAlpha < SEAM_EDITOR._forceAlphaMin) {
				SEAM_EDITOR._forceRafId = null;
				return;
			}
			SEAM_EDITOR._forceRafId = requestAnimationFrame(forceFrame);
		}
		SEAM_EDITOR._forceRafId = requestAnimationFrame(forceFrame);
	}
}

/**
 * Stops the running simulation immediately (nodes keep whatever
 * position they were last at -- this does not reset the layout).
 * Exposed as window.stopForceLayout for the same reason as
 * startForceLayout above.
 */
SEAM_EDITOR.stopForceLayout = function() {
	if (SEAM_EDITOR._forceRafId !== null) {
		cancelAnimationFrame(SEAM_EDITOR._forceRafId);
		SEAM_EDITOR._forceRafId = null;
	}
	SEAM_EDITOR._forceAlpha = 0;
}

SEAM_EDITOR.computeNodeRadius = function(node, c) {
	if(!SEAM_EDITOR.seamAutoSize || !node.text) {
		node._r = SEAM_EDITOR.nodeRadius;
		return SEAM_EDITOR.nodeRadius;
	}
	c.font = SEAM_EDITOR.seamFont;
	var lines = node.text.split('\n');
	var maxW = 0;
	for(var i = 0; i < lines.length; i++) {
		var w = c.measureText(SEAM_EDITOR.convertLatexShortcuts(lines[i])).width;
		if(w > maxW) maxW = w;
	}
	var totalH = lines.length * SEAM_EDITOR.seamLineHeight;
	// Radius must contain bounding box (maxW × totalH) with padding on all axes
	var r = Math.max(maxW / 2, totalH / 2) + SEAM_EDITOR.seamAutoSizePad;
	r = Math.max(r, SEAM_EDITOR.nodeRadius); // never smaller than default
	node._r = r;
	return r;
}

SEAM_EDITOR.drawUsing = function(c) {
	c.clearRect(0, 0, SEAM_EDITOR.canvas.width, SEAM_EDITOR.canvas.height);
	c.save();
	c.translate(SEAM_EDITOR.viewport.x + 0.5, SEAM_EDITOR.viewport.y + 0.5);
	c.scale(SEAM_EDITOR.viewport.k, SEAM_EDITOR.viewport.k);

	SEAM_EDITOR._suppressEditBox = true; // edit box drawn in overlay pass after all other elements
	for(var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		SEAM_EDITOR.computeNodeRadius(SEAM_EDITOR.nodes[i], c);
		c.lineWidth = SEAM_EDITOR.seamNodeStroke;
		if(SEAM_EDITOR.nodes[i] == SEAM_EDITOR.selectedObject) {
			c.fillStyle = c.strokeStyle = 'blue';
			SEAM_EDITOR.seamActiveLabelColor = 'blue';
		} else {
			c.fillStyle = c.strokeStyle = SEAM_EDITOR.seamNodeStrokeColor;
			SEAM_EDITOR.seamActiveLabelColor = SEAM_EDITOR.seamNodeLabelColor;
		}
		SEAM_EDITOR.nodes[i].draw(c);
	}
	// Draw halo ring around _haloNode (link-start affordance)
	if(SEAM_EDITOR._haloNode != null && SEAM_EDITOR._haloOpacity > 0.001) {
		c.save();
		c.beginPath();
		c.arc(SEAM_EDITOR._haloNode.x, SEAM_EDITOR._haloNode.y, (SEAM_EDITOR._haloNode._r != null ? SEAM_EDITOR._haloNode._r : SEAM_EDITOR.nodeRadius) + 14, 0, 2 * Math.PI);
		c.lineWidth = 14;
		c.strokeStyle = 'rgba(99,102,241,' + (0.28 * SEAM_EDITOR._haloOpacity) + ')';
		c.stroke();
		c.restore();
	}

	for(var i = 0; i < SEAM_EDITOR.links.length; i++) {
		// Skip links with stale node references (defensive guard)
		var lnk = SEAM_EDITOR.links[i];
		if((lnk instanceof SEAM_EDITOR.Link && (SEAM_EDITOR.nodes.indexOf(lnk.nodeA) < 0 || SEAM_EDITOR.nodes.indexOf(lnk.nodeB) < 0)) ||
		   ((lnk instanceof SEAM_EDITOR.SelfLink || lnk instanceof SEAM_EDITOR.StartLink) && SEAM_EDITOR.nodes.indexOf(lnk.node) < 0)) {
			SEAM_EDITOR.links.splice(i--, 1);
			continue;
		}
		c.lineWidth = SEAM_EDITOR.seamLinkStroke;
		if(SEAM_EDITOR.links[i] == SEAM_EDITOR.selectedObject) {
			c.fillStyle = c.strokeStyle = 'blue';
			SEAM_EDITOR.seamActiveLabelColor = 'blue';
		} else {
			var _lc = SEAM_EDITOR.links[i]._id ? (SEAM_EDITOR.linkColors.get(SEAM_EDITOR.links[i]._id) || null) : null;
			c.fillStyle = c.strokeStyle = _lc || SEAM_EDITOR.seamLinkStrokeColor;
			SEAM_EDITOR.seamActiveLabelColor = SEAM_EDITOR.seamLinkLabelColor;
		}
		// Halo: semi-transparent wide stroke when hovered
		var isHovered = SEAM_EDITOR.links[i] === SEAM_EDITOR._hoveredLink && SEAM_EDITOR.links[i] !== SEAM_EDITOR.selectedObject;
		if(isHovered) {
			c.save();
			c.lineWidth = SEAM_EDITOR.seamLinkStroke + 10;
			c.strokeStyle = 'rgba(99,102,241,0.14)';
			SEAM_EDITOR.links[i].drawArcOnly(c);
			c.restore();
		}
		SEAM_EDITOR.links[i].draw(c);
		// Midpoint dot: animated, appears when mouse is within 10px of line
		if(SEAM_EDITOR.links[i] !== SEAM_EDITOR.selectedObject) {
			var showDot = SEAM_EDITOR.links[i] === SEAM_EDITOR._dotLink && SEAM_EDITOR._dotOpacity > 0.005;
			if(showDot) {
				var mp = SEAM_EDITOR.links[i].getMidpoint();
				c.save();
				c.beginPath();
				c.arc(mp.x, mp.y, 3, 0, 2 * Math.PI);
				c.fillStyle = 'rgba(99,102,241,' + (0.75 * SEAM_EDITOR._dotOpacity) + ')';
				c.fill();
				c.restore();
			}
		}
	}
	if(SEAM_EDITOR.currentLink != null) {
		c.lineWidth = SEAM_EDITOR.seamLinkStroke;
		SEAM_EDITOR.seamActiveLabelColor = SEAM_EDITOR.seamLinkLabelColor;
		if(SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.TemporaryLink) {
			c.fillStyle = c.strokeStyle = SEAM_EDITOR.seamLinkStrokeColor;
			c.save(); c.globalAlpha = SEAM_EDITOR._linkPulseAlpha;
			SEAM_EDITOR.currentLink.draw(c);
			// Ghost node when in add-linked-node mode
			if(SEAM_EDITOR._addingLinkedNode) {
				var _gr = SEAM_EDITOR.nodeRadius;
				c.beginPath();
				c.arc(SEAM_EDITOR.currentLink.to.x, SEAM_EDITOR.currentLink.to.y, _gr, 0, 2 * Math.PI);
				c.fillStyle = SEAM_EDITOR.seamNodeFill;
				c.fill();
				c.strokeStyle = SEAM_EDITOR.seamNodeStrokeColor;
				c.stroke();
			}
			c.restore();
		} else {
			c.fillStyle = c.strokeStyle = SEAM_EDITOR.seamAccentColor;
			SEAM_EDITOR.currentLink.draw(c);
		}
	}
	if(SEAM_EDITOR._snapBackActive && SEAM_EDITOR._snapBackLink != null) {
		c.lineWidth = SEAM_EDITOR.seamLinkStroke;
		c.fillStyle = c.strokeStyle = SEAM_EDITOR.seamLinkStrokeColor;
		c.save(); c.globalAlpha = SEAM_EDITOR._linkPulseAlpha;
		// Draw snap-back as a TemporaryLink-shaped line+arrow
		c.beginPath();
		c.moveTo(SEAM_EDITOR._snapBackLink.to.x, SEAM_EDITOR._snapBackLink.to.y);
		c.lineTo(SEAM_EDITOR._snapBackLink.from.x, SEAM_EDITOR._snapBackLink.from.y);
		c.stroke();
		SEAM_EDITOR.drawArrow(c, SEAM_EDITOR._snapBackLink.to.x, SEAM_EDITOR._snapBackLink.to.y,
			Math.atan2(SEAM_EDITOR._snapBackLink.to.y - SEAM_EDITOR._snapBackLink.from.y, SEAM_EDITOR._snapBackLink.to.x - SEAM_EDITOR._snapBackLink.from.x));
		c.restore();
	}

	c.restore();

	// Rubber band selection rectangle (drawn in screen space after restore)
	if(SEAM_EDITOR._rectSelecting && SEAM_EDITOR._rectSelStart && SEAM_EDITOR._rectSelCur) {
		var sx1 = (SEAM_EDITOR._rectSelStart.x) * SEAM_EDITOR.viewport.k + SEAM_EDITOR.viewport.x + 0.5;
		var sy1 = (SEAM_EDITOR._rectSelStart.y) * SEAM_EDITOR.viewport.k + SEAM_EDITOR.viewport.y + 0.5;
		var sx2 = (SEAM_EDITOR._rectSelCur.x)   * SEAM_EDITOR.viewport.k + SEAM_EDITOR.viewport.x + 0.5;
		var sy2 = (SEAM_EDITOR._rectSelCur.y)   * SEAM_EDITOR.viewport.k + SEAM_EDITOR.viewport.y + 0.5;
		c.save();
		c.strokeStyle = 'rgba(99,102,241,0.7)';
		c.lineWidth = 1;
		c.setLineDash([5, 3]);
		c.fillStyle = 'rgba(99,102,241,0.07)';
		c.fillRect(Math.min(sx1,sx2), Math.min(sy1,sy2),
		           Math.abs(sx2-sx1), Math.abs(sy2-sy1));
		c.strokeRect(Math.min(sx1,sx2), Math.min(sy1,sy2),
		             Math.abs(sx2-sx1), Math.abs(sy2-sy1));
		c.setLineDash([]);
		c.restore();
	}

	// Group selection halos — pulsating indigo ring around each selected node
	if(SEAM_EDITOR._selectedNodes.length > 0) {
		c.save();
		c.translate(SEAM_EDITOR.viewport.x + 0.5, SEAM_EDITOR.viewport.y + 0.5);
		c.scale(SEAM_EDITOR.viewport.k, SEAM_EDITOR.viewport.k);
		for(var _ghi = 0; _ghi < SEAM_EDITOR._selectedNodes.length; _ghi++) {
			var _ghn = SEAM_EDITOR._selectedNodes[_ghi];
			var _ghr = (_ghn._r != null ? _ghn._r : SEAM_EDITOR.nodeRadius) + 10;
			c.beginPath();
			c.arc(_ghn.x, _ghn.y, _ghr, 0, 2 * Math.PI);
			c.lineWidth = 8;
			c.strokeStyle = 'rgba(99,102,241,' + (0.22 * SEAM_EDITOR._groupHaloAlpha) + ')';
			c.stroke();
		}
		c.restore();
	}

	// Edit overlay: draw only the edit box + text for the selected node, above everything
	if(SEAM_EDITOR.selectedObject instanceof SEAM_EDITOR.Node && SEAM_EDITOR.editingLabel && SEAM_EDITOR.canvasHasFocus() && document.hasFocus()) {
		SEAM_EDITOR._suppressEditBox = false;
		c.save();
		c.translate(SEAM_EDITOR.viewport.x + 0.5, SEAM_EDITOR.viewport.y + 0.5);
		c.scale(SEAM_EDITOR.viewport.k, SEAM_EDITOR.viewport.k);
		c.font = SEAM_EDITOR.seamFont;
		c.fillStyle = c.strokeStyle = 'blue';
		SEAM_EDITOR.seamActiveLabelColor = 'blue';
		// Draw only the label text + edit box (not circle, not accept ring)
		SEAM_EDITOR.drawText(c, SEAM_EDITOR.selectedObject.text, SEAM_EDITOR.selectedObject.x, SEAM_EDITOR.selectedObject.y, null, true);
		// Redraw accept ring after the edit box so it sits above it but ring
		// stays below the caret/text (drawn by drawText already)
		if(SEAM_EDITOR.selectedObject.isAcceptState) {
			var _ovr = SEAM_EDITOR.selectedObject._r != null ? SEAM_EDITOR.selectedObject._r : SEAM_EDITOR.nodeRadius;
			c.strokeStyle = SEAM_EDITOR.seamAccentColor;
			c.lineWidth = SEAM_EDITOR.seamNodeStroke;
			c.beginPath();
			c.arc(SEAM_EDITOR.selectedObject.x, SEAM_EDITOR.selectedObject.y, _ovr - 6, 0, 2 * Math.PI, false);
			c.stroke();
		}
		c.restore();
	}
}

SEAM_EDITOR.draw = function() {
	SEAM_EDITOR.drawUsing(SEAM_EDITOR.canvas.getContext('2d'));
}

// Nodes take priority over links when a click could hit either --
// checked in a separate, earlier pass, so a link passing near or
// through a node never wins the hit test over the node itself.
SEAM_EDITOR.selectObject = function(x, y) {
	for(var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		if(SEAM_EDITOR.nodes[i].containsPoint(x, y)) {
			return SEAM_EDITOR.nodes[i];
		}
	}
	for(var i = 0; i < SEAM_EDITOR.links.length; i++) {
		if(SEAM_EDITOR.links[i].containsPoint(x, y)) {
			return SEAM_EDITOR.links[i];
		}
	}
	return null;
}

SEAM_EDITOR.snapNode = function(node) {
	for(var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		if(SEAM_EDITOR.nodes[i] == node) continue;

		if(Math.abs(node.x - SEAM_EDITOR.nodes[i].x) < SEAM_EDITOR.snapToPadding) {
			node.x = SEAM_EDITOR.nodes[i].x;
		}

		if(Math.abs(node.y - SEAM_EDITOR.nodes[i].y) < SEAM_EDITOR.snapToPadding) {
			node.y = SEAM_EDITOR.nodes[i].y;
		}
	}
}

// Centre and scale all nodes to fit the visible canvas.
// Call after restoreFromBackupData and after canvas dimensions are set.
SEAM_EDITOR.zoomToFit = function() {
	if(!SEAM_EDITOR.canvas || SEAM_EDITOR.nodes.length === 0) return;
	var pad = 60;
	var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
	for(var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		var _nr = SEAM_EDITOR.nodes[i]._r != null ? SEAM_EDITOR.nodes[i]._r : SEAM_EDITOR.nodeRadius;
		minX = Math.min(minX, SEAM_EDITOR.nodes[i].x - _nr);
		minY = Math.min(minY, SEAM_EDITOR.nodes[i].y - _nr);
		maxX = Math.max(maxX, SEAM_EDITOR.nodes[i].x + _nr);
		maxY = Math.max(maxY, SEAM_EDITOR.nodes[i].y + _nr);
	}
	var graphW = maxX - minX + pad * 2;
	var graphH = maxY - minY + pad * 2;
	var k = Math.min(SEAM_EDITOR.canvas.width / graphW, SEAM_EDITOR.canvas.height / graphH, 1.5);
	SEAM_EDITOR.viewport.k = k;
	SEAM_EDITOR.viewport.x = SEAM_EDITOR.canvas.width  / 2 - (minX + (maxX - minX) / 2) * k;
	SEAM_EDITOR.viewport.y = SEAM_EDITOR.canvas.height / 2 - (minY + (maxY - minY) / 2) * k;
	SEAM_EDITOR.draw();
}

// Zoom by a scale factor toward the canvas centre.
// factor > 1 zooms in; factor < 1 zooms out.
SEAM_EDITOR.zoomBy = function(factor) {
	if (!SEAM_EDITOR.canvas) return;
	var newK = Math.max(0.15, Math.min(5, SEAM_EDITOR.viewport.k * factor));
	factor = newK / SEAM_EDITOR.viewport.k;
	var cx = SEAM_EDITOR.canvas.width  / 2;
	var cy = SEAM_EDITOR.canvas.height / 2;
	SEAM_EDITOR.viewport.k  = newK;
	SEAM_EDITOR.viewport.x  = cx - (cx - SEAM_EDITOR.viewport.x) * factor;
	SEAM_EDITOR.viewport.y  = cy - (cy - SEAM_EDITOR.viewport.y) * factor;
	SEAM_EDITOR.draw();
}


// ── showDialog ────────────────────────────────────────────────────────────────
// Portable Promise-based modal.
SEAM_EDITOR.showFSMDialog = function(opts) {
	return new Promise(function(resolve) {
		var dark = document.documentElement.classList.contains('dark');
		var backdrop = document.createElement('div');
		Object.assign(backdrop.style, {
			position:'fixed', inset:'0',
			background:'rgba(0,0,0,0)', backdropFilter:'blur(2px)',
			zIndex:'9999', display:'flex', alignItems:'center', justifyContent:'center',
			transition:'background 220ms ease',
		});
		var modal = document.createElement('div');
		Object.assign(modal.style, {
			background: dark ? '#1e293b' : '#ffffff',
			borderRadius:'12px', boxShadow:'0 20px 60px rgba(0,0,0,0.3)',
			width:'400px', maxWidth:'calc(100vw - 32px)',
			overflow:'hidden', fontFamily:'inherit',
			opacity:'0', transform:'scale(0.70)',
			transition:'opacity 200ms ease, transform 200ms ease',
		});
		// Header
		var header = document.createElement('div');
		Object.assign(header.style, {
			padding:'16px 20px',
			borderBottom:'1px solid ' + (dark ? '#334155' : '#e2e8f0'),
			display:'flex', alignItems:'center', justifyContent:'space-between',
		});
		var titleEl = document.createElement('div');
		Object.assign(titleEl.style, { fontWeight:'600', fontSize:'15px', color: dark ? '#f1f5f9' : '#0f172a' });
		titleEl.textContent = opts.title;
		var closeBtn = document.createElement('button');
		closeBtn.innerHTML = '<span class="material-icons" style="font-size:18px;line-height:1">close</span>';
		Object.assign(closeBtn.style, {
			background:'none', border:'none', cursor:'pointer',
			color: dark ? '#94a3b8' : '#64748b', padding:'2px', borderRadius:'4px',
			display:'flex', alignItems:'center',
		});
		function _dismiss(result) {
			document.removeEventListener('keydown', onKey);
			backdrop.style.background = 'rgba(0,0,0,0)';
			modal.style.opacity = '0';
			modal.style.transform = 'scale(0.70)';
			setTimeout(function() {
				if(backdrop.parentNode) backdrop.parentNode.removeChild(backdrop);
				resolve(result);
			}, 220);
		}
		closeBtn.onclick = function() { _dismiss(null); };
		header.append(titleEl, closeBtn);
		// Body
		var body = document.createElement('div');
		body.style.cssText = 'padding:16px 20px;display:flex;flex-direction:column;gap:14px;';
		var inputs = {};
		for(var fi = 0; fi < opts.fields.length; fi++) {
			var field = opts.fields[fi];
			var wrap = document.createElement('div');
			var lbl = document.createElement('label');
			lbl.textContent = field.label;
			Object.assign(lbl.style, {
				display:'block', fontSize:'11px', fontWeight:'500',
				color: dark ? '#94a3b8' : '#64748b',
				marginBottom:'4px', textTransform:'uppercase', letterSpacing:'0.05em',
			});
			var inp = document.createElement('input');
			inp.type = 'text';
			inp.value = field.value || '';
			inp.placeholder = field.placeholder || '';
			Object.assign(inp.style, {
				width:'100%', boxSizing:'border-box',
				padding:'7px 10px', fontSize:'13px', borderRadius:'6px',
				border:'1px solid ' + (dark ? '#475569' : '#cbd5e1'),
				background: dark ? '#0f172a' : '#f8fafc',
				color: dark ? '#e2e8f0' : '#1e293b', outline:'none',
			});
			(function(i) {
				i.addEventListener('focus', function() { i.style.borderColor='#6366f1'; i.style.boxShadow='0 0 0 3px rgba(99,102,241,0.15)'; });
				i.addEventListener('blur',  function() { i.style.borderColor = dark ? '#475569' : '#cbd5e1'; i.style.boxShadow='none'; });
				i.addEventListener('keydown', function(ev) { if(ev.key === 'Enter') { saveBtn.click(); ev.preventDefault(); } });
			})(inp);
			inputs[field.key] = inp;
			wrap.append(lbl, inp);
			body.appendChild(wrap);
		}
		// Footer
		var footer = document.createElement('div');
		Object.assign(footer.style, {
			padding:'12px 20px', borderTop:'1px solid ' + (dark ? '#334155' : '#e2e8f0'),
			display:'flex', justifyContent:'flex-end', gap:'8px',
		});
		var cancelBtn = document.createElement('button');
		cancelBtn.textContent = 'Cancel';
		Object.assign(cancelBtn.style, {
			padding:'6px 14px', fontSize:'13px', borderRadius:'6px',
			border:'1px solid ' + (dark ? '#475569' : '#cbd5e1'),
			background:'none', cursor:'pointer', color: dark ? '#94a3b8' : '#64748b',
		});
		cancelBtn.onclick = function() { _dismiss(null); };
		var saveBtn = document.createElement('button');
		saveBtn.textContent = 'Save';
		Object.assign(saveBtn.style, {
			padding:'6px 16px', fontSize:'13px', fontWeight:'600',
			borderRadius:'6px', border:'none', cursor:'pointer',
			background:'#6366f1', color:'#ffffff',
		});
		saveBtn.addEventListener('mouseover', function() { saveBtn.style.background='#4f46e5'; });
		saveBtn.addEventListener('mouseout',  function() { saveBtn.style.background='#6366f1'; });
		saveBtn.onclick = function() {
			var result = {};
			for(var k in inputs) result[k] = inputs[k].value;
			_dismiss(result);
		};
		footer.append(cancelBtn, saveBtn);
		modal.append(header, body, footer);
		backdrop.appendChild(modal);
		document.body.appendChild(backdrop);
		// Double rAF: first frame commits initial styles,
		// second frame triggers the transition from them.
		requestAnimationFrame(function() {
			requestAnimationFrame(function() {
				backdrop.style.background = 'rgba(0,0,0,0.45)';
				modal.style.opacity = '1';
				modal.style.transform = 'scale(1)';
			});
		});
		setTimeout(function() { var first = inputs[opts.fields[0].key]; if(first) first.focus(); }, 50);
		var onKey = function(ev) {
			if(ev.key === 'Escape') { _dismiss(null); }
		};
		document.addEventListener('keydown', onKey);
	});
}

// Ensure each link gets a stable _id for the properties map
SEAM_EDITOR.ensureLinkId = function(link) {
	if(!link._id) link._id = ++SEAM_EDITOR._linkIdCounter;
	return link._id;
}

SEAM_EDITOR.initFSM = function(canvasEl) {
	SEAM_EDITOR.canvas = canvasEl;

	SEAM_EDITOR.canvas.oncontextmenu = function(e) { e.preventDefault(); return false; };

	SEAM_EDITOR.canvas.onmousedown = function(e) {
		var mouse = SEAM_EDITOR.crossBrowserRelativeMousePos(e);
		SEAM_EDITOR.movingObject = false;
		SEAM_EDITOR.originalClick = mouse;
		SEAM_EDITOR.editingLabel = false;
		SEAM_EDITOR._hasDragged  = false;

		// Halo click: left=connect-link, right=add-linked-node
		if(SEAM_EDITOR._haloNode != null && SEAM_EDITOR._haloVisible && !e.shiftKey) {
			var hn = SEAM_EDITOR._haloNode;
			SEAM_EDITOR._haloNode    = null;
			SEAM_EDITOR._haloVisible = false;
			SEAM_EDITOR._haloOpacity = 0;
			SEAM_EDITOR._haloTarget  = 0;
			SEAM_EDITOR.selectedObject = hn;
			if(e.button === 2) {
				// Right-click halo: add-linked-node mode
				SEAM_EDITOR._addingLinkedNode = true;
				SEAM_EDITOR._addLinkedOrigin  = hn;
			} else {
				SEAM_EDITOR._addingLinkedNode = false;
				SEAM_EDITOR._addLinkedOrigin  = null;
			}
			SEAM_EDITOR.currentLink = new SEAM_EDITOR.TemporaryLink(hn.closestPointOnCircle(mouse.x, mouse.y), mouse);
			SEAM_EDITOR._startLinkPulse();
			SEAM_EDITOR.canvas.style.cursor = 'crosshair';
			SEAM_EDITOR.draw();
			return false;
		}

		// Clear group selection on any left-click (unless we hit a selected node)
		var _hitSelected = false;
		if(SEAM_EDITOR._selectedNodes.length > 0) {
			var _hm = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
			if(_hm instanceof SEAM_EDITOR.Node && SEAM_EDITOR._selectedNodes.indexOf(_hm) >= 0) {
				_hitSelected = true;
				// Move the whole group (any mouse button)
				SEAM_EDITOR.movingObject = true;
				SEAM_EDITOR._hasDragged = false;
				for(var _gi = 0; _gi < SEAM_EDITOR._selectedNodes.length; _gi++) {
					SEAM_EDITOR._selectedNodes[_gi].setMouseStart(mouse.x, mouse.y);
				}
				SEAM_EDITOR.canvas.style.cursor = 'grabbing';
				SEAM_EDITOR.draw();
				return false;
			}
			// Clicked away from any selected node — clear on mouseup if no drag
		}
		SEAM_EDITOR.selectedObject = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
		if(SEAM_EDITOR.selectedObject != null) {
			SEAM_EDITOR.caretPosition = (SEAM_EDITOR.selectedObject.text || '').length;
			if(e.shiftKey && SEAM_EDITOR.selectedObject instanceof SEAM_EDITOR.Node) {
				// Shift+click on node toggles accept (green) state
				SEAM_EDITOR.selectedObject.isAcceptState = !SEAM_EDITOR.selectedObject.isAcceptState;
				SEAM_EDITOR.selectedObject = null; // deselect so no label edit follows
				SEAM_EDITOR.draw();
				return false;
			} else {
				SEAM_EDITOR.movingObject = true;
				deltaMouseX = deltaMouseY = 0;
				if(SEAM_EDITOR.selectedObject.setMouseStart) {
					SEAM_EDITOR.selectedObject.setMouseStart(mouse.x, mouse.y);
				}
				SEAM_EDITOR.canvas.style.cursor = 'grabbing';
			}
			// resetCaret called only on stationary mouseup
		} else if(e.button === 2) {
			// Right-drag on empty canvas: start rubber-band group selection
			SEAM_EDITOR._rectSelecting = true;
			SEAM_EDITOR._rectSelStart  = { x: mouse.x, y: mouse.y };
			SEAM_EDITOR._rectSelCur    = { x: mouse.x, y: mouse.y };
			SEAM_EDITOR._selectedNodes = [];
			SEAM_EDITOR.canvas.style.cursor = 'crosshair';
		} else {
			// Left-drag on empty canvas: pan
			SEAM_EDITOR.panningCanvas = true;
			SEAM_EDITOR.panStart = SEAM_EDITOR.crossBrowserRawMousePos(e);
			SEAM_EDITOR.panViewStart = { x: SEAM_EDITOR.viewport.x, y: SEAM_EDITOR.viewport.y };
			SEAM_EDITOR.canvas.style.cursor = 'grabbing';
		}

		SEAM_EDITOR.draw();

		if(SEAM_EDITOR.canvasHasFocus()) {
			// disable drag-and-drop only if the canvas is already focused
			return false;
		} else {
			// otherwise, let the browser switch the focus away from wherever it was
			SEAM_EDITOR.resetCaret();
			return true;
		}
	};

	SEAM_EDITOR.canvas.ondblclick = function(e) {
		var mouse = SEAM_EDITOR.crossBrowserRelativeMousePos(e);
		SEAM_EDITOR.selectedObject = SEAM_EDITOR.selectObject(mouse.x, mouse.y);

		if(SEAM_EDITOR.selectedObject == null) {
			SEAM_EDITOR.selectedObject = new SEAM_EDITOR.Node(mouse.x, mouse.y);
			SEAM_EDITOR.nodes.push(SEAM_EDITOR.selectedObject);
			SEAM_EDITOR.resetCaret();
			SEAM_EDITOR._startNodeGrow(SEAM_EDITOR.selectedObject);
			SEAM_EDITOR.draw();
		}
	};

	SEAM_EDITOR.canvas.onmousemove = function(e) {
		var mouse = SEAM_EDITOR.crossBrowserRelativeMousePos(e);

		// Skip link tracking during snap-back animation
		if(SEAM_EDITOR._snapBackActive) return;

		if(SEAM_EDITOR.currentLink != null) {
			var hoveredNode = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
			if(!(hoveredNode instanceof SEAM_EDITOR.Node)) {
				hoveredNode = null;
			}

			// Live preview of what releasing the mouse right now would
			// create -- five distinct cases depending on whether the
			// drag started on a node (selectedObject) and whether the
			// mouse is currently over one (hoveredNode):
			if(SEAM_EDITOR.selectedObject == null) {
				// Drag started on empty space.
				if(hoveredNode != null) {
					// ...and is now over a node: a start arrow pointing into it.
					SEAM_EDITOR._stopLinkPulse();
					SEAM_EDITOR.currentLink = new SEAM_EDITOR.StartLink(hoveredNode, SEAM_EDITOR.originalClick);
				} else {
					// ...and still isn't over anything: a rubber-band line.
					SEAM_EDITOR.currentLink = new SEAM_EDITOR.TemporaryLink(SEAM_EDITOR.originalClick, mouse);
					SEAM_EDITOR._startLinkPulse();
				}
			} else {
				// Drag started on a node.
				if(hoveredNode == SEAM_EDITOR.selectedObject) {
					// ...and is back over that same node: a self-loop.
					SEAM_EDITOR._stopLinkPulse();
					SEAM_EDITOR.currentLink = new SEAM_EDITOR.SelfLink(SEAM_EDITOR.selectedObject, mouse);
				} else if(hoveredNode != null) {
					// ...and is over a different node: a link between them.
					SEAM_EDITOR._stopLinkPulse();
					SEAM_EDITOR.currentLink = new SEAM_EDITOR.Link(SEAM_EDITOR.selectedObject, hoveredNode);
				} else {
					// ...and isn't over any node: a rubber-band line from
					// the origin node's own edge (not its center) to the mouse.
					SEAM_EDITOR.currentLink = new SEAM_EDITOR.TemporaryLink(SEAM_EDITOR.selectedObject.closestPointOnCircle(mouse.x, mouse.y), mouse);
					SEAM_EDITOR._startLinkPulse();
				}
			}
			SEAM_EDITOR.draw();
		}

		if(SEAM_EDITOR.movingObject) {
			SEAM_EDITOR._hasDragged = true;
			if(SEAM_EDITOR._selectedNodes.length > 0 && SEAM_EDITOR.selectedObject == null) {
				// Group drag
				for(var _gmi = 0; _gmi < SEAM_EDITOR._selectedNodes.length; _gmi++) {
					SEAM_EDITOR._selectedNodes[_gmi].setAnchorPoint(mouse.x, mouse.y);
					// Pin dragged nodes in place while a force layout is
					// running, matching d3-force's own fx/fy convention --
					// a no-op when force layout isn't active (_forceRafId
					// stays null for every existing FSM-editing use of
					// this file).
					if(SEAM_EDITOR._forceRafId !== null) { SEAM_EDITOR._selectedNodes[_gmi].fx = SEAM_EDITOR._selectedNodes[_gmi].x; SEAM_EDITOR._selectedNodes[_gmi].fy = SEAM_EDITOR._selectedNodes[_gmi].y; }
				}
			} else if(SEAM_EDITOR.selectedObject != null) {
				SEAM_EDITOR.selectedObject.setAnchorPoint(mouse.x, mouse.y);
				if(SEAM_EDITOR.selectedObject instanceof SEAM_EDITOR.Node) {
					SEAM_EDITOR.snapNode(SEAM_EDITOR.selectedObject);
					if(SEAM_EDITOR._forceRafId !== null) { SEAM_EDITOR.selectedObject.fx = SEAM_EDITOR.selectedObject.x; SEAM_EDITOR.selectedObject.fy = SEAM_EDITOR.selectedObject.y; }
				}
			}
			SEAM_EDITOR.draw();
		}

		if(SEAM_EDITOR.panningCanvas) {
			var raw = SEAM_EDITOR.crossBrowserRawMousePos(e);
			SEAM_EDITOR.viewport.x = SEAM_EDITOR.panViewStart.x + (raw.x - SEAM_EDITOR.panStart.x);
			SEAM_EDITOR.viewport.y = SEAM_EDITOR.panViewStart.y + (raw.y - SEAM_EDITOR.panStart.y);
			SEAM_EDITOR.draw();
		}
		if(SEAM_EDITOR._rectSelecting) {
			SEAM_EDITOR._hasDragged = true;
			SEAM_EDITOR._rectSelCur = { x: mouse.x, y: mouse.y };
			SEAM_EDITOR.draw();
		}

		// Hover cursor and halo state when nothing is being dragged
		if(!SEAM_EDITOR.movingObject && !SEAM_EDITOR.panningCanvas && SEAM_EDITOR.currentLink == null) {
			var onNode = null;
			for(var _ni = 0; _ni < SEAM_EDITOR.nodes.length; _ni++) {
				if(SEAM_EDITOR.nodes[_ni].containsPoint(mouse.x, mouse.y)) { onNode = SEAM_EDITOR.nodes[_ni]; break; }
			}
			if(onNode != null) {
				// Inside a node: prime _haloNode, start fade-out if visible
				SEAM_EDITOR._haloNode    = onNode;
				SEAM_EDITOR._haloVisible = false;
				if(SEAM_EDITOR._haloTarget !== 0) { SEAM_EDITOR._haloTarget = 0; SEAM_EDITOR._startHaloAnim(); }
				if(SEAM_EDITOR._hoveredLink !== null) { SEAM_EDITOR._hoveredLink = null; SEAM_EDITOR.draw(); }
				if(SEAM_EDITOR._dotTarget !== 0) { SEAM_EDITOR._dotTarget = 0; SEAM_EDITOR._startDotAnim(); }
				SEAM_EDITOR.canvas.style.cursor = 'grab';
			} else if(SEAM_EDITOR._haloNode != null) {
				// Outside all nodes: check halo zone distance
				var _dx = mouse.x - SEAM_EDITOR._haloNode.x, _dy = mouse.y - SEAM_EDITOR._haloNode.y;
				var _hr = SEAM_EDITOR._haloNode._r != null ? SEAM_EDITOR._haloNode._r : SEAM_EDITOR.nodeRadius;
				var _dist = Math.sqrt(_dx*_dx + _dy*_dy) - _hr;
				if(_dist > 0 && _dist <= 20) {
					SEAM_EDITOR._haloVisible = true;
					if(SEAM_EDITOR._haloTarget !== 1) { SEAM_EDITOR._haloTarget = 1; SEAM_EDITOR._startHaloAnim(); }
					SEAM_EDITOR.canvas.style.cursor = 'cell';
					if(SEAM_EDITOR._hoveredLink !== null) { SEAM_EDITOR._hoveredLink = null; SEAM_EDITOR.draw(); }
				} else {
					SEAM_EDITOR._haloVisible = false;
					if(SEAM_EDITOR._haloTarget !== 0) { SEAM_EDITOR._haloTarget = 0; SEAM_EDITOR._startHaloAnim(); }
					var _hov = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
					SEAM_EDITOR.canvas.style.cursor = (_hov != null) ? 'grab' : 'crosshair';
				}
			} else {
				var _hov2 = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
				SEAM_EDITOR.canvas.style.cursor = (_hov2 != null) ? 'grab' : 'crosshair';
			}
			// _hoveredLink and dot: unconditional within idle block
			var _hlCheck = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
			var _nl = (_hlCheck instanceof SEAM_EDITOR.Link || _hlCheck instanceof SEAM_EDITOR.SelfLink || _hlCheck instanceof SEAM_EDITOR.StartLink) ? _hlCheck : null;
			if(_nl !== SEAM_EDITOR._hoveredLink) { SEAM_EDITOR._hoveredLink = _nl; SEAM_EDITOR.draw(); }
			// Dot proximity check: unconditional, 30px tolerance
			var _oldPad = SEAM_EDITOR.hitTargetPadding;
			SEAM_EDITOR.hitTargetPadding = 30;
			var _near = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
			SEAM_EDITOR.hitTargetPadding = _oldPad;
			var _dotCandidate = (_near instanceof SEAM_EDITOR.Link || _near instanceof SEAM_EDITOR.SelfLink || _near instanceof SEAM_EDITOR.StartLink) ? _near : null;
			if(_dotCandidate !== SEAM_EDITOR._dotLink) {
				if(_dotCandidate != null) {
					SEAM_EDITOR._dotLink = _dotCandidate;
					SEAM_EDITOR._dotTarget = 1;
				} else {
					SEAM_EDITOR._dotTarget = 0;
				}
				SEAM_EDITOR._startDotAnim();
			}
		}
	};

	SEAM_EDITOR.canvas.onmouseup = function(e) {
		var didDrag = SEAM_EDITOR._hasDragged;
		SEAM_EDITOR.movingObject = false;
		SEAM_EDITOR.panningCanvas = false;
		// Release any node pinned in place for dragging during an active
		// force layout (see onmousemove) -- no-op when force layout isn't
		// running.
		if(SEAM_EDITOR._forceRafId !== null) {
			if(SEAM_EDITOR.selectedObject instanceof SEAM_EDITOR.Node) { SEAM_EDITOR.selectedObject.fx = null; SEAM_EDITOR.selectedObject.fy = null; }
			for(var _rgmi = 0; _rgmi < SEAM_EDITOR._selectedNodes.length; _rgmi++) { SEAM_EDITOR._selectedNodes[_rgmi].fx = null; SEAM_EDITOR._selectedNodes[_rgmi].fy = null; }
		}
		if(SEAM_EDITOR._rectSelecting) {
			SEAM_EDITOR._rectSelecting = false;
			// Collect nodes inside rect
			if(SEAM_EDITOR._rectSelStart && SEAM_EDITOR._rectSelCur) {
				var rx1 = Math.min(SEAM_EDITOR._rectSelStart.x, SEAM_EDITOR._rectSelCur.x);
				var ry1 = Math.min(SEAM_EDITOR._rectSelStart.y, SEAM_EDITOR._rectSelCur.y);
				var rx2 = Math.max(SEAM_EDITOR._rectSelStart.x, SEAM_EDITOR._rectSelCur.x);
				var ry2 = Math.max(SEAM_EDITOR._rectSelStart.y, SEAM_EDITOR._rectSelCur.y);
				SEAM_EDITOR._selectedNodes = [];
				for(var _ri = 0; _ri < SEAM_EDITOR.nodes.length; _ri++) {
					var _rn = SEAM_EDITOR.nodes[_ri];
					if(_rn.x >= rx1 && _rn.x <= rx2 && _rn.y >= ry1 && _rn.y <= ry2) {
						SEAM_EDITOR._selectedNodes.push(_rn);
					}
				}
				if(SEAM_EDITOR._selectedNodes.length > 0) SEAM_EDITOR._startGroupHalo();
			}
			SEAM_EDITOR._rectSelStart = SEAM_EDITOR._rectSelCur = null;
			SEAM_EDITOR.canvas.style.cursor = 'crosshair';
			SEAM_EDITOR.draw();
			return;
		}
		SEAM_EDITOR._hasDragged = false;

		// Any stationary click clears group selection
		if(!didDrag && SEAM_EDITOR._selectedNodes.length > 0) {
			SEAM_EDITOR._selectedNodes = [];
			SEAM_EDITOR._groupHaloRafId = null;
			SEAM_EDITOR.draw();
		}

		// Shift+click on a committed link: cycle its colour
		if(!didDrag && e.shiftKey && SEAM_EDITOR.currentLink == null &&
		   SEAM_EDITOR.selectedObject != null && !(SEAM_EDITOR.selectedObject instanceof SEAM_EDITOR.Node)) {
			SEAM_EDITOR.ensureLinkId(SEAM_EDITOR.selectedObject);
			var _cycle = [SEAM_EDITOR.seamAccentColor, '#4b0082', '#8b0000', null];
			var _cur = SEAM_EDITOR.linkColors.get(SEAM_EDITOR.selectedObject._id) || null;
			var _idx = _cycle.indexOf(_cur);
			var _next = _cycle[(_idx + 1) % _cycle.length];
			if(_next === null) SEAM_EDITOR.linkColors.delete(SEAM_EDITOR.selectedObject._id);
			else SEAM_EDITOR.linkColors.set(SEAM_EDITOR.selectedObject._id, _next);
			SEAM_EDITOR.selectedObject = null; // deselect so custom colour is immediately visible
			SEAM_EDITOR.draw();
		// Activate label editing only on a stationary click with no link being committed
		} else if(!didDrag && !e.shiftKey && SEAM_EDITOR.currentLink == null &&
		          SEAM_EDITOR.selectedObject != null && 'text' in SEAM_EDITOR.selectedObject) {
			SEAM_EDITOR.editingLabel = true;
			SEAM_EDITOR.resetCaret();
		}

		// Always restore cursor based on what is under the mouse
		var hovered = SEAM_EDITOR.selectObject(SEAM_EDITOR.crossBrowserRelativeMousePos(e).x, SEAM_EDITOR.crossBrowserRelativeMousePos(e).y);
		SEAM_EDITOR.canvas.style.cursor = (hovered != null) ? 'grab' : 'crosshair';

			if(SEAM_EDITOR._addingLinkedNode && SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.TemporaryLink) {
				SEAM_EDITOR._addingLinkedNode = false;
				var _dropTarget = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
				if(_dropTarget instanceof SEAM_EDITOR.Node && _dropTarget !== SEAM_EDITOR._addLinkedOrigin) {
					// Dropped on existing node — commit as normal link
					SEAM_EDITOR._stopLinkPulse();
					var _newLink = new SEAM_EDITOR.Link(SEAM_EDITOR._addLinkedOrigin, _dropTarget);
					SEAM_EDITOR.links.push(_newLink);
					SEAM_EDITOR.selectedObject = _newLink;
					SEAM_EDITOR.resetCaret();
				} else {
					// Dropped on empty canvas — create new node + link
					SEAM_EDITOR._stopLinkPulse();
					var _newNode = new SEAM_EDITOR.Node(mouse.x, mouse.y);
					SEAM_EDITOR.nodes.push(_newNode);
					var _newLink2 = new SEAM_EDITOR.Link(SEAM_EDITOR._addLinkedOrigin, _newNode);
					SEAM_EDITOR.links.push(_newLink2);
					SEAM_EDITOR.selectedObject = _newNode;
					SEAM_EDITOR.resetCaret();
					SEAM_EDITOR._startNodeGrow(_newNode);
				}
				SEAM_EDITOR._addLinkedOrigin = null;
				SEAM_EDITOR.currentLink = null;
				SEAM_EDITOR.draw();
			} else if(SEAM_EDITOR.currentLink != null) {
				if(SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.TemporaryLink) {
					// Released without connecting: snap back to origin
					SEAM_EDITOR._startSnapBack(SEAM_EDITOR.currentLink);
				} else {
					SEAM_EDITOR._stopLinkPulse();
					// Validate all node references before committing
					var valid = true;
					if(SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.Link) {
						valid = SEAM_EDITOR.nodes.indexOf(SEAM_EDITOR.currentLink.nodeA) >= 0 && SEAM_EDITOR.nodes.indexOf(SEAM_EDITOR.currentLink.nodeB) >= 0;
					} else if(SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.SelfLink) {
						valid = SEAM_EDITOR.nodes.indexOf(SEAM_EDITOR.currentLink.node) >= 0;
					} else if(SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.StartLink) {
						valid = SEAM_EDITOR.nodes.indexOf(SEAM_EDITOR.currentLink.node) >= 0;
					}
					if(valid) {
						SEAM_EDITOR.selectedObject = SEAM_EDITOR.currentLink;
						SEAM_EDITOR.links.push(SEAM_EDITOR.currentLink);
						SEAM_EDITOR.resetCaret();
					}
					SEAM_EDITOR.currentLink = null;
					SEAM_EDITOR.draw();
				}
			}
	};

	// Cancel drag/pan/link if mouse leaves canvas
	SEAM_EDITOR.canvas.onmouseleave = function() {
		SEAM_EDITOR._haloVisible = false;
		if(SEAM_EDITOR._haloTarget !== 0) { SEAM_EDITOR._haloTarget = 0; SEAM_EDITOR._startHaloAnim(); }
		var _leaveRedraw = SEAM_EDITOR._hoveredLink !== null;
		SEAM_EDITOR._hoveredLink = null;
		if(SEAM_EDITOR._dotTarget !== 0) { SEAM_EDITOR._dotTarget = 0; SEAM_EDITOR._startDotAnim(); }
		if(SEAM_EDITOR.currentLink instanceof SEAM_EDITOR.TemporaryLink) {
			SEAM_EDITOR._startSnapBack(SEAM_EDITOR.currentLink);
			SEAM_EDITOR.movingObject = false;
			SEAM_EDITOR.panningCanvas = false;
			SEAM_EDITOR.canvas.style.cursor = 'crosshair';
		} else if(SEAM_EDITOR.movingObject || SEAM_EDITOR.panningCanvas) {
			SEAM_EDITOR.movingObject = false;
			SEAM_EDITOR.panningCanvas = false;
			SEAM_EDITOR.currentLink = null;
			SEAM_EDITOR.canvas.style.cursor = 'crosshair';
			SEAM_EDITOR.draw();
		} else if(_leaveRedraw) {
			SEAM_EDITOR.draw();
		}
	};

	// Right-click on a link: open transition properties dialog
	SEAM_EDITOR.canvas.oncontextmenu = function(e) {
		e.preventDefault();
		var mouse = SEAM_EDITOR.crossBrowserRelativeMousePos(e);
		var target = SEAM_EDITOR.selectObject(mouse.x, mouse.y);
		if(target == null || target instanceof SEAM_EDITOR.Node) return;
		SEAM_EDITOR.ensureLinkId(target);
		var props = SEAM_EDITOR.linkProperties.get(target._id) || { guard: '', action: '' };
		SEAM_EDITOR.showFSMDialog({
			title: 'Transition Properties',
			fields: [
				{ key: 'label',  label: 'Label',  value: target.text,   placeholder: 'e.g. unlock' },
				{ key: 'guard',  label: 'Guard',  value: props.guard,  placeholder: 'e.g. key_valid == true' },
				{ key: 'action', label: 'Action', value: props.action, placeholder: 'e.g. notify:email' },
			],
		}).then(function(result) {
			if(result == null) return;
			target.text = result.label;
			SEAM_EDITOR.linkProperties.set(target._id, { guard: result.guard, action: result.action });
			SEAM_EDITOR.draw();
		});
	};

	// Wheel zoom with inertia — accumulates velocity in log-space,
	// then decays via requestAnimationFrame for a smooth continuous feel.
	SEAM_EDITOR.canvas.onwheel = function(e) {
		e.preventDefault();
		var raw = SEAM_EDITOR.crossBrowserRawMousePos(e);
		// Cursor position: update on each event so fast flicks track correctly
		SEAM_EDITOR._zoomCursorX = raw.x;
		SEAM_EDITOR._zoomCursorY = raw.y;
		// Accumulate in log-space: uniform feel at any zoom level
		var sensitivity = 0.0008;
		SEAM_EDITOR._zoomVelocity += e.deltaY * sensitivity;
		if(SEAM_EDITOR._zoomRafId === null) {
			var lastTime = performance.now();
			function zoomFrame(now) {
				var dt = Math.min(now - lastTime, 64); // cap dt to avoid jumps after tab switch
				lastTime = now;
				var step = SEAM_EDITOR._zoomVelocity * dt * 0.05;
				if(Math.abs(step) > 0.0001) {
					var factor = Math.exp(-step);
					var newK = Math.max(0.15, Math.min(5, SEAM_EDITOR.viewport.k * factor));
					factor = newK / SEAM_EDITOR.viewport.k;
					SEAM_EDITOR.viewport.k = newK;
					SEAM_EDITOR.viewport.x = SEAM_EDITOR._zoomCursorX - (SEAM_EDITOR._zoomCursorX - SEAM_EDITOR.viewport.x) * factor;
					SEAM_EDITOR.viewport.y = SEAM_EDITOR._zoomCursorY - (SEAM_EDITOR._zoomCursorY - SEAM_EDITOR.viewport.y) * factor;
					SEAM_EDITOR.draw();
					SEAM_EDITOR._zoomVelocity *= 0.801; // decay (tuned for ~10% longer inertia than 0.78)
					SEAM_EDITOR._zoomRafId = requestAnimationFrame(zoomFrame);
				} else {
					SEAM_EDITOR._zoomVelocity = 0;
					SEAM_EDITOR._zoomRafId = null;
				}
			}
			SEAM_EDITOR._zoomRafId = requestAnimationFrame(zoomFrame);
		}
		return false;
	};
}

// Consolidated three separate listeners (keydown/keyup/keypress, all
// built on the deprecated e.which/e.keyCode via crossBrowserKey) into
// one keydown handler using e.key strings. Three real things this
// fixes, found by reading the prior implementation directly rather
// than assumed:
//
// 1. The old keypress-based character insertion checked
//    `key >= 0x20 && key <= 0x7E` -- an ASCII-only range. Any non-ASCII
//    character (accented letters, non-Latin scripts) could never be
//    typed into an FSM label. e.key gives the actual typed character
//    directly, for any script, so `key.length === 1` is both the
//    simpler check and the one that actually works for non-ASCII input.
//
// 2. The old keyup listener computed `key` and did nothing with it --
//    dead code, removed outright.
//
// 3. The more serious one: an earlier version of this handler
//    registered via `document.onkeydown = function(e) {...}` (DOM0
//    assignment), where `return false` genuinely prevents the
//    browser's default action. At some point this became
//    `document.addEventListener('keydown', ...)` -- a reasonable
//    change on its own (doesn't clobber another handler elsewhere in
//    the app) -- but `return false` from an addEventListener callback
//    does NOT prevent default browser behaviour; only e.preventDefault()
//    does. That means the "backspace doesn't navigate back" and
//    "space doesn't scroll the page" protections the comments describe
//    were not actually working -- pressing Backspace with nothing
//    selected could genuinely trigger the browser's back button and
//    lose unsaved work. A regression introduced when the registration
//    style changed without updating for it. Fixed here with real
//    e.preventDefault() calls everywhere a default action needs
//    blocking.
document.addEventListener('keydown', function(e) {
	if (!SEAM_EDITOR.canvas) return;
	if (!SEAM_EDITOR.canvasHasFocus()) {
		// don't read keystrokes when other things have focus
		return true;
	}

	var key = e.key;
	var isEditingText = SEAM_EDITOR.selectedObject != null && 'text' in SEAM_EDITOR.selectedObject && SEAM_EDITOR.editingLabel;

	if (key === 'Backspace') {
		if (isEditingText && SEAM_EDITOR.caretPosition > 0) {
			var t = SEAM_EDITOR.selectedObject.text;
			SEAM_EDITOR.selectedObject.text = t.substring(0, SEAM_EDITOR.caretPosition - 1) + t.substring(SEAM_EDITOR.caretPosition);
			SEAM_EDITOR.caretPosition = Math.max(0, SEAM_EDITOR.caretPosition - 1);
			SEAM_EDITOR.resetCaret();
			SEAM_EDITOR.draw();
		} else if (SEAM_EDITOR.selectedObject != null && !SEAM_EDITOR.editingLabel) {
			// Not editing: backspace deletes the selected node/link (same as Delete)
			for (var _bi = 0; _bi < SEAM_EDITOR.nodes.length; _bi++) {
				if (SEAM_EDITOR.nodes[_bi] == SEAM_EDITOR.selectedObject) { SEAM_EDITOR.nodes.splice(_bi--, 1); }
			}
			for (_bi = 0; _bi < SEAM_EDITOR.links.length; _bi++) {
				if (SEAM_EDITOR.links[_bi] == SEAM_EDITOR.selectedObject || SEAM_EDITOR.links[_bi].node == SEAM_EDITOR.selectedObject ||
					SEAM_EDITOR.links[_bi].nodeA == SEAM_EDITOR.selectedObject || SEAM_EDITOR.links[_bi].nodeB == SEAM_EDITOR.selectedObject) {
					SEAM_EDITOR.links.splice(_bi--, 1);
				}
			}
			SEAM_EDITOR.selectedObject = null;
			SEAM_EDITOR.editingLabel = false;
			SEAM_EDITOR.draw();
		}
		e.preventDefault(); // Backspace also navigates back by default -- must not change pages
	} else if (key === 'Enter') {
		if (e.altKey && isEditingText) {
			var t2 = SEAM_EDITOR.selectedObject.text || '';
			SEAM_EDITOR.selectedObject.text = t2.substring(0, SEAM_EDITOR.caretPosition) + '\n' + t2.substring(SEAM_EDITOR.caretPosition);
			SEAM_EDITOR.caretPosition++;
			SEAM_EDITOR.resetCaret(); SEAM_EDITOR.draw();
			e.preventDefault();
		} else {
			SEAM_EDITOR.selectedObject = null; SEAM_EDITOR.editingLabel = false; SEAM_EDITOR.caretPosition = 0; SEAM_EDITOR.draw();
		}
	} else if (key === 'Escape') {
		SEAM_EDITOR.selectedObject = null; SEAM_EDITOR.editingLabel = false; SEAM_EDITOR.caretPosition = 0; SEAM_EDITOR.draw();
	} else if (key === 'ArrowLeft') {
		if (isEditingText) {
			SEAM_EDITOR.caretPosition = Math.max(0, SEAM_EDITOR.caretPosition - 1);
			SEAM_EDITOR.resetCaret(); SEAM_EDITOR.draw();
			e.preventDefault();
		}
	} else if (key === 'ArrowRight') {
		if (isEditingText) {
			SEAM_EDITOR.caretPosition = Math.min((SEAM_EDITOR.selectedObject.text || '').length, SEAM_EDITOR.caretPosition + 1);
			SEAM_EDITOR.resetCaret(); SEAM_EDITOR.draw();
			e.preventDefault();
		}
	} else if (key === 'Home') {
		if (isEditingText) {
			SEAM_EDITOR.caretPosition = 0;
			SEAM_EDITOR.resetCaret(); SEAM_EDITOR.draw();
			e.preventDefault();
		}
	} else if (key === 'End') {
		if (isEditingText) {
			SEAM_EDITOR.caretPosition = (SEAM_EDITOR.selectedObject.text || '').length;
			SEAM_EDITOR.resetCaret(); SEAM_EDITOR.draw();
			e.preventDefault();
		}
	} else if (key === 'ArrowUp') {
		if (isEditingText) {
			var tUp = SEAM_EDITOR.selectedObject.text || '';
			var lcUp = SEAM_EDITOR.caretToLineCol(tUp, SEAM_EDITOR.caretPosition);
			if (lcUp.line > 0) SEAM_EDITOR.caretPosition = SEAM_EDITOR.lineColToPos(tUp, lcUp.line - 1, lcUp.col);
			SEAM_EDITOR.resetCaret(); SEAM_EDITOR.draw();
			e.preventDefault();
		}
	} else if (key === 'ArrowDown') {
		if (isEditingText) {
			var tDown = SEAM_EDITOR.selectedObject.text || '';
			var lcDown = SEAM_EDITOR.caretToLineCol(tDown, SEAM_EDITOR.caretPosition);
			if (lcDown.line < tDown.split('\n').length - 1) SEAM_EDITOR.caretPosition = SEAM_EDITOR.lineColToPos(tDown, lcDown.line + 1, lcDown.col);
			SEAM_EDITOR.resetCaret(); SEAM_EDITOR.draw();
			e.preventDefault();
		}
	} else if (key === 'Delete') {
		if (SEAM_EDITOR.selectedObject != null) {
			for (var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
				if (SEAM_EDITOR.nodes[i] == SEAM_EDITOR.selectedObject) { SEAM_EDITOR.nodes.splice(i--, 1); }
			}
			for (i = 0; i < SEAM_EDITOR.links.length; i++) {
				if (SEAM_EDITOR.links[i] == SEAM_EDITOR.selectedObject || SEAM_EDITOR.links[i].node == SEAM_EDITOR.selectedObject || SEAM_EDITOR.links[i].nodeA == SEAM_EDITOR.selectedObject || SEAM_EDITOR.links[i].nodeB == SEAM_EDITOR.selectedObject) {
					SEAM_EDITOR.links.splice(i--, 1);
				}
			}
			SEAM_EDITOR.selectedObject = null;
			SEAM_EDITOR.editingLabel = false;
			SEAM_EDITOR.draw();
		}
	} else if (key.length === 1 && !e.metaKey && !e.altKey && !e.ctrlKey && isEditingText) {
		// A single printable character. e.key gives the real typed
		// character for any script/layout -- see point 1 in the comment
		// above for why this replaces the old ASCII-range check, not
		// just modernizes its spelling.
		var tIns = SEAM_EDITOR.selectedObject.text || '';
		SEAM_EDITOR.selectedObject.text = tIns.substring(0, SEAM_EDITOR.caretPosition) + key + tIns.substring(SEAM_EDITOR.caretPosition);
		SEAM_EDITOR.caretPosition++;
		SEAM_EDITOR.resetCaret();
		SEAM_EDITOR.draw();
		e.preventDefault(); // space scrolls the page, etc. by default -- must not do that while typing a label
	}
});

// Raw canvas-space position using e.offsetX/Y (reliable in embedded context).
// An older offsetParent-traversal approach fails inside complex layouts.
SEAM_EDITOR.crossBrowserRawMousePos = function(e) {
	return { 'x': e.offsetX, 'y': e.offsetY };
}

// World-space position (corrected for viewport pan and zoom).
SEAM_EDITOR.crossBrowserRelativeMousePos = function(e) {
	var raw = SEAM_EDITOR.crossBrowserRawMousePos(e);
	return {
		'x': (raw.x - SEAM_EDITOR.viewport.x) / SEAM_EDITOR.viewport.k,
		'y': (raw.y - SEAM_EDITOR.viewport.y) / SEAM_EDITOR.viewport.k
	};
}

SEAM_EDITOR.output = function(text) {
	var element = document.getElementById('output');
	if (!element) return;
	element.style.display = 'block';
	element.value = text;
}





// Compute world-space bounding box from node geometry — reliable regardless
// of canvas size or current zoom/pan. Returns [minX, minY, maxX, maxY].
SEAM_EDITOR.getGeometryBounds = function() {
	if(!SEAM_EDITOR.nodes.length) return [0, 0, SEAM_EDITOR.canvas.width || 600, SEAM_EDITOR.canvas.height || 400];
	var pad = 60;
	var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
	for(var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		var r = SEAM_EDITOR.nodes[i]._r != null ? SEAM_EDITOR.nodes[i]._r : SEAM_EDITOR.nodeRadius;
		minX = Math.min(minX, SEAM_EDITOR.nodes[i].x - r);
		minY = Math.min(minY, SEAM_EDITOR.nodes[i].y - r);
		maxX = Math.max(maxX, SEAM_EDITOR.nodes[i].x + r);
		maxY = Math.max(maxY, SEAM_EDITOR.nodes[i].y + r);
	}
	// Expand for link labels and arrowheads
	minX -= pad; minY -= pad; maxX += pad; maxY += pad;
	return [minX, minY, maxX, maxY];
}

// Returns the FSM as an SVG string at world scale (viewport reset to identity).\n// Clears transient visual state (halo, dot) during export so they do not appear.
SEAM_EDITOR.getSVGString = function() {
	var oldSelected = SEAM_EDITOR.selectedObject;
	var savedViewport = { x: SEAM_EDITOR.viewport.x, y: SEAM_EDITOR.viewport.y, k: SEAM_EDITOR.viewport.k };
	var savedHalo = SEAM_EDITOR._haloOpacity, savedDot = SEAM_EDITOR._dotOpacity;
	SEAM_EDITOR.selectedObject = null;
	SEAM_EDITOR.viewport = { x: 0, y: 0, k: 1 };
	SEAM_EDITOR._haloOpacity = 0; SEAM_EDITOR._dotOpacity = 0;
	// Use geometry-based bounds — not pixel scan — so the full graph is captured
	// regardless of canvas size or how far nodes extend beyond the visible area.
	var bounds = SEAM_EDITOR.getGeometryBounds();
	// Render to SVG exporter
	var exporter = new SEAM_EDITOR.ExportAsSVG(bounds);
	SEAM_EDITOR.drawUsing(exporter);
	var svg = exporter.toSVG();
	// Restore state
	SEAM_EDITOR.selectedObject = oldSelected;
	SEAM_EDITOR.viewport = savedViewport;
	SEAM_EDITOR._haloOpacity = savedHalo; SEAM_EDITOR._dotOpacity = savedDot;
	SEAM_EDITOR.draw(); // restore visual
	return svg;
}


// Returns nothing (triggers a browser download); saves and restores
// viewport/selection/halo/dot state around the export the same way
// getSVGString() does above. Correction found by comparing the two
// side by side: this function used to zero _haloOpacity/_dotOpacity
// without saving their prior value, so exporting to LaTeX during an
// active halo or dot animation permanently killed it -- SVG export
// never had this problem, since getSVGString() already saved and
// restored both. It also never touched viewport at all. LaTeX's own
// translate()/scale() are no-ops, so this was not currently visible as
// a rendering bug (world-space coordinates don't depend on viewport),
// but it left LaTeX export one future viewport-dependent rendering
// change away from silently diverging from SVG export's own behavior.
// Matching the defensive pattern here costs nothing and removes both
// risks at once.
SEAM_EDITOR.saveAsLaTeX = function(monochrome) {
	var bounds = SEAM_EDITOR.getGeometryBounds();
	var exporter = new SEAM_EDITOR.ExportAsLaTeX(bounds, monochrome);
	var oldSelectedObject = SEAM_EDITOR.selectedObject;
	var savedViewport = { x: SEAM_EDITOR.viewport.x, y: SEAM_EDITOR.viewport.y, k: SEAM_EDITOR.viewport.k };
	var savedHalo = SEAM_EDITOR._haloOpacity, savedDot = SEAM_EDITOR._dotOpacity;
	SEAM_EDITOR.selectedObject = null;
	SEAM_EDITOR.viewport = { x: 0, y: 0, k: 1 };
	SEAM_EDITOR._haloOpacity = 0; SEAM_EDITOR._dotOpacity = 0;
	SEAM_EDITOR.drawUsing(exporter);
	SEAM_EDITOR.selectedObject = oldSelectedObject;
	SEAM_EDITOR.viewport = savedViewport;
	SEAM_EDITOR._haloOpacity = savedHalo; SEAM_EDITOR._dotOpacity = savedDot;
	SEAM_EDITOR.draw();
	var texData = exporter.toLaTeX();
	var blob = new Blob([texData], { type: 'application/x-latex' });
	var url = URL.createObjectURL(blob);
	var a = document.createElement('a');
	a.href = url;
	a.download = monochrome ? 'fsm-diagram-mono.tex' : 'fsm-diagram.tex';
	document.body.appendChild(a); a.click(); document.body.removeChild(a);
	URL.revokeObjectURL(url);
}




// 3x3 matrix determinant via cofactor expansion along the top row.
// Used below to solve for a circle through three points.
SEAM_EDITOR.determinant3x3 = function(m11, m12, m13, m21, m22, m23, m31, m32, m33) {
	return m11*m22*m33 + m12*m23*m31 + m13*m21*m32 - m11*m23*m32 - m12*m21*m33 - m13*m22*m31;
}

// The unique circle passing through three given points -- the classic
// circumcircle-from-determinants construction: a circle
// x^2+y^2+Dx+Ey+F=0 through (x1,y1), (x2,y2), (x3,y3) has D, E, F (and
// therefore center/radius) expressible as ratios of determinants built
// from those three points, each expanded here via determinant3x3 above.
// Note bx and by are NOT symmetric in sign (bx negated, by not) -- a
// genuine asymmetry in this derivation, not a typo; verified against
// the original numerically before relying on it.
SEAM_EDITOR.circleFromThreePoints = function(x1, y1, x2, y2, x3, y3) {
	var a = SEAM_EDITOR.determinant3x3(x1, y1, 1, x2, y2, 1, x3, y3, 1);
	var bx = -SEAM_EDITOR.determinant3x3(x1*x1 + y1*y1, y1, 1, x2*x2 + y2*y2, y2, 1, x3*x3 + y3*y3, y3, 1);
	var by = SEAM_EDITOR.determinant3x3(x1*x1 + y1*y1, x1, 1, x2*x2 + y2*y2, x2, 1, x3*x3 + y3*y3, x3, 1);
	var c = -SEAM_EDITOR.determinant3x3(x1*x1 + y1*y1, x1, y1, x2*x2 + y2*y2, x2, y2, x3*x3 + y3*y3, x3, y3);
	return {
		x: -bx / (2*a),
		y: -by / (2*a),
		radius: Math.sqrt(bx*bx + by*by - 4*a*c) / (2*Math.abs(a))
	};
}

// Trims a fixed-decimal number string down to its significant digits --
// "3.00" becomes "3", "3.10" becomes "3.1". Used by the LaTeX/SVG
// export code, where exact-width decimal output would look noisy.
SEAM_EDITOR.fixed = function(number, digits) {
	return number.toFixed(digits).replace(/0+$/, '').replace(/\.$/, '');
}

SEAM_EDITOR.restoreFromBackupData = function(backup) {
	for(var i = 0; i < backup.nodes.length; i++) {
		var backupNode = backup.nodes[i];
		var node = new SEAM_EDITOR.Node(backupNode.x, backupNode.y);
		node.isAcceptState = backupNode.isAcceptState;
		node.text = backupNode.text;
		node.textOnly = backupNode.textOnly;
		SEAM_EDITOR.nodes.push(node);
	}
	for(var i = 0; i < backup.links.length; i++) {
		var backupLink = backup.links[i];
		var link = null;
		switch(backupLink.type) {
			case 'SelfLink':
				link = new SEAM_EDITOR.SelfLink(SEAM_EDITOR.nodes[backupLink.node]);
				link.seam_anchorAngle = backupLink.anchorAngle;
				link.text = backupLink.text;
				break;
			case 'StartLink':
				link = new SEAM_EDITOR.StartLink(SEAM_EDITOR.nodes[backupLink.node]);
				link.seam_deltaX = backupLink.deltaX;
				link.seam_deltaY = backupLink.deltaY;
				link.text = backupLink.text;
				break;
			case 'Link':
				link = new SEAM_EDITOR.Link(SEAM_EDITOR.nodes[backupLink.nodeA], SEAM_EDITOR.nodes[backupLink.nodeB]);
				link.parallelPart = backupLink.parallelPart;
				link.perpendicularPart = backupLink.perpendicularPart;
				link.text = backupLink.text;
				link.lineAngleAdjust = backupLink.lineAngleAdjust;
				break;
		}
		if(link != null) {
			SEAM_EDITOR.links.push(link);
			if(backupLink.guard || backupLink.action || backupLink.color) {
				SEAM_EDITOR.ensureLinkId(link);
				SEAM_EDITOR.linkProperties.set(link._id, { guard: backupLink.guard || '', action: backupLink.action || '' });
				if(backupLink.color) SEAM_EDITOR.linkColors.set(link._id, backupLink.color);
			}
		}
	}
	SEAM_EDITOR.nodeRadius = backup.nodeRadius;
	SEAM_EDITOR.draw();
}


SEAM_EDITOR.getBackupData = function() {
	var backup = {
		nodes: [],
		links: [],
		nodeRadius: SEAM_EDITOR.nodeRadius
	};
	for(var i = 0; i < SEAM_EDITOR.nodes.length; i++) {
		var node = SEAM_EDITOR.nodes[i];
		backup.nodes.push({
			x: node.x,
			y: node.y,
			text: node.text,
			isAcceptState: node.isAcceptState,
			textOnly: node.textOnly
		});
	}
	for(var i = 0; i < SEAM_EDITOR.links.length; i++) {
		var link = SEAM_EDITOR.links[i];
		var linkProps = link._id ? (SEAM_EDITOR.linkProperties.get(link._id) || {}) : {};
		var linkColor = link._id ? (SEAM_EDITOR.linkColors.get(link._id) || null) : null;
		var backupLink = null;
		if(link instanceof SEAM_EDITOR.SelfLink) {
			backupLink = {
				type: 'SelfLink',
				node: SEAM_EDITOR.nodes.indexOf(link.node),
				text: link.text,
				anchorAngle: link.seam_anchorAngle
			};
		} else if(link instanceof SEAM_EDITOR.StartLink) {
			backupLink = {
				type: 'StartLink',
				node: SEAM_EDITOR.nodes.indexOf(link.node),
				text: link.text,
				deltaX: link.seam_deltaX,
				deltaY: link.seam_deltaY
			};
		} else if(link instanceof SEAM_EDITOR.Link) {
			backupLink = {
				type: 'Link',
				nodeA: SEAM_EDITOR.nodes.indexOf(link.nodeA),
				nodeB: SEAM_EDITOR.nodes.indexOf(link.nodeB),
				text: link.text,
				lineAngleAdjust: link.lineAngleAdjust,
				parallelPart: link.parallelPart,
				perpendicularPart: link.perpendicularPart
			};
		}
		if(backupLink != null) {
			// guard/action/color are generic across every link type, so
			// they are applied once here rather than duplicated in each
			// of the three branches above -- matching how
			// restoreFromBackupData already applies them back in one
			// shared place rather than three.
			backupLink.guard = linkProps.guard || '';
			backupLink.action = linkProps.action || '';
			backupLink.color = linkColor || '';
			backup.links.push(backupLink);
		}
	}
	return backup;
}


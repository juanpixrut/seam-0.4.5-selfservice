/**
 * Seam File Upload — Lit shell
 *
 * Backs the formengine "image" widget (T-10). Dropzone.js (vendored,
 * MIT — web/static/vendor/dropzone@5.9.3.min.js) provides the
 * drag-and-drop drop zone, upload progress bars, and thumbnail
 * previews-before-upload; this shell owns the widget's own state (the
 * gallery's current item list) and talks to the backend
 * (internal/ui/media_gallery.go) for listing, uploading, and deleting
 * items. Same shell-plus-engine split fsm-editor.js's SeamFSMEditor
 * already uses for the FSM canvas engine — the person's own
 * direction for this widget ("wrapping it with Lit, similar to the
 * way we've done it with other components before").
 *
 * Dropzone's own <script src="..."> and <link rel="stylesheet"> tags
 * are NOT emitted from this file's render() output -- renderImage
 * (widgets.go) emits them in the surrounding Go-rendered HTML instead.
 * A <script> inserted via a Lit template is inert (script elements
 * created through template cloning, which is how lit-html renders,
 * never execute -- a real bug caught and fixed while building this,
 * not a hypothetical): only a <script> present in the originally
 * parsed document actually runs.
 *
 * Lit is imported by direct URL, not the bare 'lit' specifier
 * fsm-editor.js uses — that specifier only resolves via the page-level
 * import map layout.go's LitImportMap sets on specific routes (the FSM
 * editor page). This widget can appear on any formengine-rendered
 * page, and a second <script type="importmap"> added after any module
 * script has already run throws in current browsers — the direct URL
 * import sidesteps that entirely: no import map needed, and a schema
 * with more than one image field on the same page just works, each
 * instance importing the same URL (the browser's own module cache
 * dedupes the fetch).
 *
 * Custom element registration is guarded (customElements.get check
 * before define) for the same multi-instance-per-page reason.
 */
import { LitElement, html } from '/static/vendor/lit@3.js';

if (!customElements.get('seam-file-upload')) {

class SeamFileUpload extends LitElement {
  static properties = {
    ownerType:  { type: String, attribute: 'owner-type' },
    ownerId:    { type: String, attribute: 'owner-id' },
    ownerField: { type: String, attribute: 'owner-field' },
    csrfToken:  { type: String, attribute: 'csrf-token' },
    labelAdd:   { type: String, attribute: 'label-add' },
    labelHint:  { type: String, attribute: 'label-hint' },
    _items:     { state: true },
    _error:     { state: true },
    _loading:   { state: true },
  };

  createRenderRoot() { return this; }

  constructor() {
    super();
    this._items = [];
    this._error = '';
    this._loading = true;
  }

  async firstUpdated() {
    await this._loadItems();
    this._initDropzone();
  }

  async _loadItems() {
    this._loading = true;
    try {
      const params = new URLSearchParams({
        owner_type: this.ownerType,
        owner_id: this.ownerId,
        owner_field: this.ownerField,
      });
      const res = await fetch(`/gallery/items?${params}`);
      if (!res.ok) throw new Error(`list failed: ${res.status}`);
      this._items = await res.json() || [];
      this._error = '';
    } catch (e) {
      this._error = (window.SeamText ? SeamText('upload-load-failed', 'Could not load existing images.') : 'Could not load existing images.');
    } finally {
      this._loading = false;
    }
  }

  async _initDropzone() {
    let dz = this.querySelector('.seam-dropzone');
    if (!dz || dz.dropzoneInstance) return; // guard against double-init on re-render
    if (!window.Dropzone) {
      // Inside a modal, the form's <script src> elements are re-created by SeamModal and load in
      // parallel with this module, in no fixed order, so Dropzone may simply not be there yet: wait
      // for it (SeamWidgets.need polls briefly, then loads the script itself) instead of giving up.
      try {
        if (window.SeamWidgets) await window.SeamWidgets.need('Dropzone', '/static/vendor/dropzone@5.9.3.min.js');
      } catch (e) { /* reported below */ }
      dz = this.querySelector('.seam-dropzone'); // the element may have re-rendered while we waited
      if (!dz || dz.dropzoneInstance) return;
    }
    if (!window.Dropzone) {
      this._error = 'Upload is unavailable (Dropzone did not load).';
      return;
    }

    const self = this;
    const instance = new window.Dropzone(dz, {
      url: '/gallery/upload',
      paramName: 'file',
      maxFilesize: 20, // MB — matches media_gallery.go's own maxGalleryUploadBytes
      acceptedFiles: 'image/*',
      addRemoveLinks: false, // this widget's own delete button handles removal, not Dropzone's
      headers: { 'X-CSRF-Token': this.csrfToken },
      params: {
        owner_type: this.ownerType,
        owner_id: this.ownerId,
        owner_field: this.ownerField,
      },
      init: function () {
        this.on('success', (file, response) => {
          self._items = [...self._items, response];
          this.removeFile(file); // uploaded item now lives in the gallery grid above, not Dropzone's own preview list
        });
        this.on('error', (file, message) => {
          self._error = typeof message === 'string' ? message : (window.SeamText ? SeamText('upload-failed', 'Upload failed.') : 'Upload failed.');
          this.removeFile(file);
        });
      },
    });
    dz.dropzoneInstance = instance;
    // Dropzone opens the file chooser only when the click's target is the drop area itself. The CSS
    // above keeps the icon, the label and the hint from taking clicks, but that must not be the only
    // thing the button relies on (a browser that disagrees, or a stylesheet from another build still
    // in its cache, left the "Add photos" button and its icon dead while the empty area worked):
    // forward any click that does land on one of them. No double open: Dropzone ignores those clicks.
    dz.addEventListener('click', (e) => {
      if (e.target !== dz && instance.hiddenFileInput) instance.hiddenFileInput.click();
    });
  }

  // Clicking a thumbnail shows the photo larger, in front of everything (the form is itself in a
  // modal, whose transform would trap a fixed element, so the overlay lives on <body>). Escape or a
  // click closes it; Escape is taken in the capture phase so it does not also close the form's modal.
  _openPreview(item) {
    this._closePreview();
    const overlay = document.createElement('div');
    overlay.className = 'seam-lightbox';
    overlay.setAttribute('role', 'dialog');
    overlay.setAttribute('aria-modal', 'true');
    Object.assign(overlay.style, {
      position: 'fixed', inset: '0', zIndex: '10050', background: 'rgba(0,0,0,0.82)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '24px', cursor: 'zoom-out',
    });
    const img = document.createElement('img');
    img.src = item.url;
    img.alt = '';
    Object.assign(img.style, {
      maxWidth: '100%', maxHeight: '100%', objectFit: 'contain', borderRadius: '8px',
      boxShadow: '0 20px 60px rgba(0,0,0,0.5)', cursor: 'default',
    });
    img.addEventListener('click', (e) => e.stopPropagation());
    const close = document.createElement('button');
    close.type = 'button';
    close.textContent = '\u00d7';
    close.title = 'Close';
    close.setAttribute('aria-label', 'Close');
    Object.assign(close.style, {
      position: 'absolute', top: '16px', right: '20px', width: '40px', height: '40px', border: '0',
      borderRadius: '9999px', background: 'rgba(255,255,255,0.18)', color: '#fff', fontSize: '28px',
      lineHeight: '1', cursor: 'pointer',
    });
    overlay.addEventListener('click', () => this._closePreview());
    overlay.append(img, close);
    this._onPreviewKey = (e) => {
      if (e.key === 'Escape') { e.stopPropagation(); e.preventDefault(); this._closePreview(); }
    };
    document.addEventListener('keydown', this._onPreviewKey, true);
    document.body.appendChild(overlay);
    this._lightbox = overlay;
    close.focus();
  }

  _closePreview() {
    if (this._lightbox) { this._lightbox.remove(); this._lightbox = null; }
    if (this._onPreviewKey) { document.removeEventListener('keydown', this._onPreviewKey, true); this._onPreviewKey = null; }
  }

  disconnectedCallback() {
    this._closePreview();
    super.disconnectedCallback();
  }

  async _deleteItem(id) {
    try {
      const res = await fetch(`/gallery/item?id=${encodeURIComponent(id)}`, {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': this.csrfToken },
      });
      if (!res.ok) throw new Error(`delete failed: ${res.status}`);
      this._items = this._items.filter((it) => it.id !== id);
    } catch (e) {
      this._error = (window.SeamText ? SeamText('upload-delete-failed', 'Could not delete image.') : 'Could not delete image.');
    }
  }

  render() {
    return html`
      <style>
        seam-file-upload { display: block; }
        .seam-gallery-grid {
          display: grid;
          grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
          gap: 8px;
          margin-bottom: 12px;
        }
        .seam-gallery-item { position: relative; }
        .seam-gallery-item img {
          width: 100%; height: 96px; object-fit: cover;
          border-radius: 8px; border: 1px solid #e2e8f0;
          cursor: pointer;
        }
        .seam-gallery-delete {
          position: absolute; top: 4px; right: 4px;
          background: rgba(0,0,0,0.6); color: #fff; border: none;
          border-radius: 9999px; width: 22px; height: 22px;
          font-size: 14px; line-height: 1; cursor: pointer;
        }
        .seam-dropzone {
          display: flex; flex-direction: column; align-items: center; gap: 6px;
          border: 2px dashed var(--fe-border, #cbd5e1); border-radius: 8px;
          padding: 20px 16px; text-align: center; font-size: 13px;
          color: var(--fe-text, #64748b); cursor: pointer;
        }
        .seam-dropzone:hover, .seam-dropzone.dz-drag-hover {
          background: var(--fe-accent-bg, rgba(79, 70, 229, 0.08));
          border-color: var(--fe-accent, #4f46e5);
        }
        .seam-dz-icon { font-size: 32px; color: var(--fe-accent, #4f46e5); }
        .seam-dz-label {
          display: inline-block; padding: 6px 14px; border-radius: 8px;
          font-weight: 500; font-size: 14px;
          border: 1px solid var(--fe-accent, #4f46e5); color: var(--fe-accent, #4f46e5);
        }
        .seam-dz-hint { opacity: 0.75; }
        /* Dropzone opens the file chooser only for a click on the drop area itself: the icon, the
           label and the hint must not take the click away from it. */
        .seam-dropzone > * { pointer-events: none; }
        .seam-error { color: #dc2626; font-size: 13px; margin-top: 8px; }
      </style>

      ${this._loading
        ? html`<p style="font-size:13px;color:#94a3b8;">Loading…</p>`
        : html`
          ${this._items.length > 0
            ? html`
              <div class="seam-gallery-grid">
                ${this._items.map(
                  (item) => html`
                    <div class="seam-gallery-item">
                      <img src="${item.url}" alt="" loading="lazy" tabindex="0" role="button"
                        @click=${() => this._openPreview(item)}
                        @keydown=${(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); this._openPreview(item); } }} />
                      <button
                        type="button"
                        class="seam-gallery-delete"
                        @click=${() => this._deleteItem(item.id)}
                        title="Remove"
                      >×</button>
                    </div>
                  `
                )}
              </div>
            `
            : ''}
          <div class="seam-dropzone dz-clickable" tabindex="0" role="button"
               @keydown=${(e) => {
                 if ((e.key === 'Enter' || e.key === ' ') && e.currentTarget.dropzoneInstance) {
                   e.preventDefault();
                   e.currentTarget.dropzoneInstance.hiddenFileInput.click();
                 }
               }}>
            <span class="material-icons seam-dz-icon" aria-hidden="true">add_a_photo</span>
            <span class="seam-dz-label">${this.labelAdd || 'Add photos'}</span>
            <span class="seam-dz-hint">${this.labelHint || 'or drag them here'}</span>
          </div>
        `}
      ${this._error ? html`<p class="seam-error">${this._error}</p>` : ''}
    `;
  }
}

customElements.define('seam-file-upload', SeamFileUpload);

}

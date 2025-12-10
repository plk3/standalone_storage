const API_URL = '/api';
let allFiles = []; // Currently loaded files
let currentPage = 1;
const ITEMS_PER_PAGE = 50;
let currentPreviewIndex = -1;
let isFetching = false;
let availableTags = [];
let lastFocusedInput = null;

// Track active input
document.addEventListener('focusin', (e) => {
    // Ignore the sidebar filter logic itself
    if (e.target.tagName === 'INPUT' && e.target.type === 'text' && e.target.id !== 'tagFilter') {
        lastFocusedInput = e.target;
    }
});

// Load Tags
async function loadTags() {
    try {
        const res = await fetch(`${API_URL}/tags`);
        if (res.ok) {
            availableTags = await res.json();
            filterTags(); // Initial render with sort and no filter
        }
    } catch (e) { console.error("Failed to load tags", e); }
}

// Render Side Panel Tags
function renderTagsPanel(tags) {
    const container = document.getElementById('sidebar-tag-list');
    container.innerHTML = tags.map(t => 
        `<span class="sidebar-tag" onclick="addTagToActiveField('${t.name}')">
            ${t.name} <span style="font-size:0.7em; opacity:0.7;">(${t.count})</span>
        </span>`
    ).join('');
}

function filterTags() {
    const query = document.getElementById('tagFilter').value.toLowerCase();
    
    // 1. Filter
    let filtered = availableTags.filter(t => t.name.toLowerCase().includes(query));

    // 2. Sort
    const sortType = document.querySelector('input[name="sortTags"]:checked').value;
    filtered.sort((a, b) => {
        if (sortType === 'count') {
            // Count desc, then Name asc
            if (b.count !== a.count) return b.count - a.count;
            return a.name.localeCompare(b.name);
        } else {
            // Name asc
            return a.name.localeCompare(b.name);
        }
    });

    renderTagsPanel(filtered);
}

window.addTagToActiveField = (tag) => {
    if (!lastFocusedInput || !document.body.contains(lastFocusedInput)) {
        showToast("Please select an input field first");
        return;
    }

    let currentVal = lastFocusedInput.value;
    const isTagInput = lastFocusedInput.id.toLowerCase().includes('tag');
    
    if (currentVal.length > 0) {
        currentVal = currentVal.trimEnd();
        if (isTagInput) {
            if (!currentVal.endsWith(',')) {
                currentVal += ',';
            }
            currentVal += ' ';
        } else {
            currentVal += ' ';
        }
    }
    
    lastFocusedInput.value = currentVal + tag;
    
    // Trigger events for frameworks/debouncers
    lastFocusedInput.dispatchEvent(new Event('input', { bubbles: true }));
    lastFocusedInput.dispatchEvent(new KeyboardEvent('keyup', { bubbles: true }));
};

function showToast(msg) {
    const x = document.getElementById("toast");
    x.textContent = msg;
    x.className = "show";
    setTimeout(function(){ x.className = x.className.replace("show", ""); }, 3000);
}

window.toggleSidebar = () => {
    document.getElementById('tag-sidebar').classList.toggle('open');
    document.body.classList.toggle('sidebar-active');
};

// Upload
document.getElementById('uploadForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const file = document.getElementById('fileInput').files[0];
    const formData = new FormData();
    formData.append('file', file);
    formData.append('tags', document.getElementById('tagsInput').value);

    try {
        const res = await fetch(`${API_URL}/upload`, { method: 'POST', body: formData });
        if (res.ok) {
            document.getElementById('uploadForm').reset();
            loadFiles(true); // Reset to page 1
            loadTags(); // Reload tags in case new ones were added
        } else alert('Upload failed');
    } catch (err) { alert('Error uploading'); }
});

// Load Files
async function loadFiles(reset = false) {
    if (isFetching) return;
    isFetching = true;
    document.getElementById('loading-indicator').style.display = 'inline';
    
    if (reset) {
        currentPage = 1;
        allFiles = [];
        document.getElementById('fileGrid').innerHTML = '';
    }

    const query = document.getElementById('searchInput').value;
    const btnLoadMore = document.getElementById('load-more-container');

    try {
        const res = await fetch(`${API_URL}/files?q=${encodeURIComponent(query)}&page=${currentPage}&limit=${ITEMS_PER_PAGE}`);
        const newFiles = await res.json();
        
            if (newFiles.length > 0) {
                allFiles = allFiles.concat(newFiles);
                renderGrid(newFiles);
                currentPage++;
            } else {
                // No more files
                if (reset) {
                    document.getElementById('fileGrid').innerHTML = '<p style="text-align:center;grid-column:1/-1;">No files found.</p>';
                }
            }
        } catch (err) { 
            console.error(err); 
        } finally {
            isFetching = false;
            document.getElementById('loading-indicator').style.display = 'none';
        }
}

function loadMore() {
    loadFiles(false);
}

// Infinite Scroll Observer
const observer = new IntersectionObserver((entries) => {
    if (entries[0].isIntersecting && !isFetching && allFiles.length > 0) {
        loadFiles(false);
    }
}, { threshold: 0.1 });

// Will start observing after initial load or in logic
document.addEventListener('DOMContentLoaded', () => {
     const sentinel = document.getElementById('sentinel');
     if(sentinel) observer.observe(sentinel);
});

function renderGrid(files) {
    const grid = document.getElementById('fileGrid');
    
    files.forEach(file => {
        const isImage = file.content_type && file.content_type.startsWith('image/');
        const isVideo = file.content_type && file.content_type.startsWith('video/');
        
        let previewContent = `<div class="type-icon">📄</div>`;
        if (isImage) {
            previewContent = `<img src="${file.url}" alt="${file.filename}" loading="lazy" onclick="previewFile('${file.id}', event)" title="Click to view full image">`;
        } else if (isVideo) {
             previewContent = `<video src="${file.url}" muted playsinline loop onmouseover="this.play()" onmouseout="this.pause()" onclick="previewFile('${file.id}', event)" title="Click to play video" style="width:100%;height:100%;object-fit:cover;"></video>`;
        }

        const card = document.createElement('div');
        card.className = 'card';
        card.id = `card-${file.id}`;
        card.innerHTML = `
            <div class="card-preview">
                ${previewContent}
            </div>
            <div class="card-body" id="body-${file.id}">
                <div class="filename" title="${file.filename}">${file.filename}</div>


                <div class="tags">
                    ${(file.tags || []).map(t => `<span class="tag-badge">${t}</span>`).join('')}
                </div>
            </div>
            <!-- Edit Form (Hidden by default) -->
            <div class="card-body editing" id="edit-${file.id}" style="display:none;">

                <input type="text" id="edit-tags-${file.id}" value="${(file.tags || []).join(', ')}" placeholder="Tags">
            </div>

            <div class="card-actions">
                <button class="btn-secondary btn-sm" onclick="toggleEdit('${file.id}')" id="btn-edit-${file.id}">Edit</button>
                <button class="btn-primary btn-sm" onclick="saveEdit('${file.id}')" id="btn-save-${file.id}" style="display:none;">Save</button>
                <button class="btn-danger btn-sm" onclick="deleteFile('${file.id}')">Delete</button>
                <a href="${file.url}" class="btn-primary btn-sm" style="text-decoration:none;" download>⬇</a>
            </div>
        `;
        grid.appendChild(card);
    });
}

// Toggle Edit
window.toggleEdit = (id) => {
    const body = document.getElementById(`body-${id}`);
    const edit = document.getElementById(`edit-${id}`);
    const btnEdit = document.getElementById(`btn-edit-${id}`);
    const btnSave = document.getElementById(`btn-save-${id}`);

    if (edit.style.display === 'none') {
        // Enter Edit Mode
        body.style.display = 'none';
        edit.style.display = 'flex';
        btnEdit.textContent = 'Cancel';
        btnSave.style.display = 'inline-block';
    } else {
        // Cancel Edit Mode
        body.style.display = 'flex';
        edit.style.display = 'none';
        btnEdit.textContent = 'Edit';
        btnSave.style.display = 'none';
    }
};

// Save Edit
window.saveEdit = async (id) => {
    const tagsStr = document.getElementById(`edit-tags-${id}`).value;
    const tags = tagsStr.split(',').map(t => t.trim()).filter(t => t);

    try {
        const res = await fetch(`${API_URL}/files/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tags: tags })
        });

        if (res.ok) {
            loadFiles(true);
            loadTags(); // Refresh tags
        } else {
            alert('Failed to update');
        }
    } catch (err) { alert('Error updating'); }
};

// Delete
window.deleteFile = async (id) => {
    if (!confirm('Are you sure you want to delete this file?')) return;
    try {
        const res = await fetch(`${API_URL}/files/${id}`, { method: 'DELETE' });
        if (res.ok) {
            document.getElementById(`card-${id}`).remove();
            allFiles = allFiles.filter(f => f.id !== id);
        } else {
            alert('Failed to delete');
        }
    } catch (err) { alert('Error deleting'); }
};

// Search
let timeout = null;
function debounceSearch() {
    clearTimeout(timeout);
    timeout = setTimeout(() => {
        loadFiles(true);
    }, 300);
}

loadFiles(true);
loadTags();

// --- Modal Logic ---
window.previewFile = async (id, event) => {
    if (event) event.stopPropagation(); 
    const index = allFiles.findIndex(f => f.id === id);
    if (index !== -1) {
        showImage(index);
        document.getElementById('preview-modal').style.display = 'flex';
    }
};

async function showImage(index) {
    if (index < 0 || index >= allFiles.length) return;
    currentPreviewIndex = index;
    const file = allFiles[index];
    const modalImg = document.getElementById('modal-img');
    const modalVideo = document.getElementById('modal-video');
    const caption = document.getElementById('modal-caption');

    // Reset previous
    modalImg.style.display = 'none';
    modalVideo.style.display = 'none';
    modalVideo.pause();
    modalVideo.src = "";
    
    try {
        const res = await fetch(`${API_URL}/files/${file.id}/download?preview=true`);
        const data = await res.json();
        const url = data.url || file.url;

        caption.innerText = `${file.filename} (${index + 1} / ${allFiles.length})`;

        if (file.content_type && file.content_type.startsWith('video/')) {
            modalVideo.src = url;
            modalVideo.style.display = 'block';
            modalVideo.play().catch(e => console.log("Auto-play failed:", e));
        } else {
            modalImg.src = url;
            modalImg.style.display = 'block';
        }

    } catch(e) {
        console.error(e);
        // Fallback
        if (file.content_type && file.content_type.startsWith('video/')) {
            modalVideo.src = file.url;
            modalVideo.style.display = 'block';
        } else {
            modalImg.src = file.url;
            modalImg.style.display = 'block';
        }
    }
}

window.closeModal = () => {
    document.getElementById('preview-modal').style.display = 'none';
    document.getElementById('modal-img').src = '';
    const v = document.getElementById('modal-video');
    v.pause();
    v.src = "";
};

window.nextImage = async (e) => {
    if (e) e.stopPropagation();
    
    // Check if we need to load more files (threshold: 5 items from end)
    if (currentPreviewIndex >= allFiles.length - 5) {
        // Trigger load in background
        if (!isFetching) {
             console.log("Pre-fetching more images...");
             loadFiles(false);
        }
    }

    // Wait if we are at the very end and loading is happening
    if (currentPreviewIndex >= allFiles.length - 1) {
        if (isFetching) {
            // Wait for fetch to complete (simple poll)
            while (isFetching) {
                await new Promise(r => setTimeout(r, 100));
            }
        }
    }

    if (currentPreviewIndex < allFiles.length - 1) {
        showImage(currentPreviewIndex + 1);
    } else {
        showToast("No more images to load");
    }
};

window.prevImage = (e) => {
    if (e) e.stopPropagation();
    if (currentPreviewIndex > 0) {
        showImage(currentPreviewIndex - 1);
    }
};

document.addEventListener('keydown', (e) => {
    if (document.getElementById('preview-modal').style.display === 'flex') {
        if (e.key === 'ArrowRight') nextImage();
        if (e.key === 'ArrowLeft') prevImage();
        if (e.key === 'Escape') closeModal();
    }
});

// Global Paste Handler
document.addEventListener('paste', (e) => {
    const items = (e.clipboardData || e.originalEvent.clipboardData).items;
    for (let i = 0; i < items.length; i++) {
        const item = items[i];
        if (item.kind === 'file' && item.type.startsWith('image/')) {
            const blob = item.getAsFile();
            const fileInput = document.getElementById('fileInput');
            
            // Create a file with a timestamp name if it doesn't have one (blob usually doesn't)
            const file = new File([blob], `pasted_image_${Date.now()}.png`, { type: blob.type });

            const dataTransfer = new DataTransfer();
            dataTransfer.items.add(file);
            fileInput.files = dataTransfer.files;
            
            showToast("Image pasted from clipboard!");
            
            // Scroll to top
            document.querySelector('.upload-card').scrollIntoView({ behavior: 'smooth' });
            // Focus description for quick entry
            document.getElementById('descInput').focus();
            
            return; // Stop after first image
        }
    }
});

// Backup
window.downloadBackup = () => {
    window.location.href = `${API_URL}/backup`;
};

// Restore
window.uploadRestore = async () => {
    const input = document.getElementById('restoreInput');
    if (!input.files || input.files.length === 0) return;
    
    if (!confirm("Restoring will overwrite existing files with the same name. Continue?")) {
        input.value = ""; // clear selection
        return;
    }

    const file = input.files[0];
    const formData = new FormData();
    formData.append('file', file);

    const btn = document.querySelector('button[onclick*="restoreInput"]');
    const originalText = btn.textContent;
    btn.textContent = "Restoring...";
    btn.disabled = true;

    try {
        const res = await fetch(`${API_URL}/restore`, {
            method: 'POST',
            body: formData
        });
        
        const data = await res.json();
        
        if (res.ok) {
            alert(data.message || "Restore successful");
            loadFiles(true);
            loadTags();
        } else {
            alert("Restore failed: " + (data.error || "Unknown error"));
        }
    } catch (e) {
        console.error(e);
        alert("Restore error");
    } finally {
        input.value = ""; // clear
        btn.textContent = originalText;
        btn.disabled = false;
    }
};

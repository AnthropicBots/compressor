(() => {
  "use strict";

  let mode = "auto";
  let algorithm = "auto";
  const history = [];
  let historySeq = 0;

  // ---------------------------------------------------------------
  // Theme
  // ---------------------------------------------------------------
  const root = document.documentElement;
  const themeToggle = document.getElementById("theme-toggle");
  const iconSun = document.getElementById("icon-sun");
  const iconMoon = document.getElementById("icon-moon");

  function applyTheme(theme) {
    root.setAttribute("data-theme", theme);
    iconSun.style.display = theme === "dark" ? "block" : "none";
    iconMoon.style.display = theme === "dark" ? "none" : "block";
    try { localStorage.setItem("huffman-theme", theme); } catch (e) {}
  }
  (function initTheme() {
    let saved = null;
    try { saved = localStorage.getItem("huffman-theme"); } catch (e) {}
    if (!saved) saved = window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    applyTheme(saved);
  })();
  themeToggle.addEventListener("click", () => {
    applyTheme(root.getAttribute("data-theme") === "dark" ? "light" : "dark");
    if (lastCompressPayload) drawTree(lastCompressPayload.analysis && lastCompressPayload.analysis.tree);
  });

  // ---------------------------------------------------------------
  // Mode + algorithm
  // ---------------------------------------------------------------
  const modeButtons = document.querySelectorAll(".tab-btn");
  const modeHint = document.getElementById("mode-hint");
  const algoRow = document.getElementById("algo-row");
  const algoSelect = document.getElementById("algo-select");
  const modeHints = {
    auto: 'Drop any file — we\'ll pick the best algorithm automatically. Drop a <code>.huff</code> file — we\'ll restore it.',
    compress: "Every file dropped here will be compressed with the chosen algorithm.",
    decompress: "Every file dropped here will be treated as a <code>.huff</code> archive and restored.",
  };

  modeButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
      modeButtons.forEach((b) => { b.classList.remove("active"); b.setAttribute("aria-selected", "false"); });
      btn.classList.add("active");
      btn.setAttribute("aria-selected", "true");
      mode = btn.dataset.mode;
      modeHint.innerHTML = modeHints[mode];
      algoRow.classList.toggle("disabled", mode === "decompress");
    });
  });
  algoSelect.addEventListener("change", () => { algorithm = algoSelect.value; });

  // ---------------------------------------------------------------
  // Dropzone
  // ---------------------------------------------------------------
  const dropzone = document.getElementById("dropzone");
  const fileInput = document.getElementById("file-input");
  dropzone.addEventListener("click", () => fileInput.click());
  dropzone.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); fileInput.click(); } });
  ["dragenter", "dragover"].forEach((evt) => dropzone.addEventListener(evt, (e) => { e.preventDefault(); dropzone.classList.add("dragover"); }));
  ["dragleave", "drop"].forEach((evt) => dropzone.addEventListener(evt, (e) => { e.preventDefault(); dropzone.classList.remove("dragover"); }));
  dropzone.addEventListener("drop", (e) => { const f = e.dataTransfer && e.dataTransfer.files; if (f && f.length) enqueueFiles(f); });
  fileInput.addEventListener("change", () => { if (fileInput.files && fileInput.files.length) enqueueFiles(fileInput.files); fileInput.value = ""; });

  // ---------------------------------------------------------------
  // Queue
  // ---------------------------------------------------------------
  const queueCard = document.getElementById("queue-card");
  const queueList = document.getElementById("queue-list");
  let queueRunning = false;
  const pendingQueue = [];

  function enqueueFiles(fileList) {
    Array.from(fileList).forEach((f) => pendingQueue.push(f));
    renderQueue();
    if (!queueRunning) runQueue();
  }
  function renderQueue() {
    queueCard.hidden = pendingQueue.length === 0 && !queueRunning;
    queueList.innerHTML = "";
    pendingQueue.forEach((file) => {
      const li = document.createElement("li");
      li.className = "queue-item";
      li.innerHTML = `<span class="qi-icon">⏳</span><span class="qi-name">${escapeHtml(file.name)}</span><span class="qi-status">waiting</span>`;
      queueList.appendChild(li);
    });
  }
  async function runQueue() {
    queueRunning = true;
    while (pendingQueue.length) {
      const file = pendingQueue.shift();
      renderQueue();
      queueCard.hidden = false;
      const activeLi = document.createElement("li");
      activeLi.className = "queue-item";
      activeLi.innerHTML = `<span class="spinner"></span><span class="qi-name">${escapeHtml(file.name)}</span><span class="qi-status">processing…</span>`;
      queueList.prepend(activeLi);
      try { await processFile(file); }
      catch (err) { showToast("error", `${file.name}: ${err.message || err}`); addHistory({ name: file.name, kind: "error", error: String(err.message || err) }); }
      activeLi.remove();
    }
    queueRunning = false;
    renderQueue();
  }

  function resolveAction(file) {
    if (mode === "compress") return "compress";
    if (mode === "decompress") return "decompress";
    return file.name.toLowerCase().endsWith(".huff") ? "decompress" : "compress";
  }

  async function processFile(file) {
    const action = resolveAction(file);
    const form = new FormData();
    form.append("file", file, file.name);
    if (action === "compress") form.append("algorithm", algorithm);

    const res = await fetch(`/api/${action}`, { method: "POST", body: form });
    let payload;
    try { payload = await res.json(); } catch (e) { throw new Error("Server returned an invalid response"); }
    if (!res.ok) throw new Error(payload.error || `Request failed (${res.status})`);

    if (action === "compress") {
      renderCompressResult(payload);
      addHistory({ name: payload.filename, kind: "compress", originalSize: payload.originalSize, resultSize: payload.compressedSize, payload });
      showToast("success", `Compressed ${payload.filename} with ${payload.algorithm} — saved ${formatPercent(payload.spaceSavedPercent)}`);
    } else {
      renderDecompressResult(payload);
      addHistory({ name: payload.filename, kind: "decompress", originalSize: payload.decompressedSize, resultSize: payload.decompressedSize, payload });
      if (payload.checksumVerified) showToast("success", `Restored ${payload.downloadName} (${payload.algorithm}) — integrity verified ✓`);
      else showToast("info", `Restored ${payload.downloadName} — checksum mismatch, please double-check`);
    }
  }

  // ---------------------------------------------------------------
  // DOM refs
  // ---------------------------------------------------------------
  const emptyPanel = document.getElementById("empty-panel");
  const resultPanel = document.getElementById("result-panel");
  const resultFilename = document.getElementById("result-filename");
  const resultSubtitle = document.getElementById("result-subtitle");
  const downloadBtn = document.getElementById("download-btn");

  const statOriginal = document.getElementById("stat-original");
  const statCompressed = document.getElementById("stat-compressed");
  const statSaved = document.getElementById("stat-saved");
  const statAlgo = document.getElementById("stat-algo");
  const statEntropy = document.getElementById("stat-entropy");
  const statEfficiency = document.getElementById("stat-efficiency");
  const statTime = document.getElementById("stat-time");
  const statChecksum = document.getElementById("stat-checksum");
  const checksumWarning = document.getElementById("checksum-warning");

  const barOriginal = document.getElementById("bar-original");
  const barCompressed = document.getElementById("bar-compressed");
  const barTheoretical = document.getElementById("bar-theoretical");
  const barOriginalValue = document.getElementById("bar-original-value");
  const barCompressedValue = document.getElementById("bar-compressed-value");
  const barTheoreticalValue = document.getElementById("bar-theoretical-value");

  const barsCard = document.getElementById("bars-card");
  const algoCompareCard = document.getElementById("algo-compare-card");
  const algoCompare = document.getElementById("algo-compare");
  const treeCard = document.getElementById("tree-card");
  const treeHint = document.getElementById("tree-hint");
  const treeHolder = document.getElementById("tree-svg-holder");
  const histCard = document.getElementById("hist-card");
  const histogramEl = document.getElementById("histogram");
  const codesCard = document.getElementById("codes-card");
  const codesTbody = document.getElementById("codes-tbody");
  const copyCodesBtn = document.getElementById("copy-codes");

  let currentDownload = null;
  let currentCodes = null;
  let lastCompressPayload = null;

  function showResultPanel() { emptyPanel.hidden = true; resultPanel.hidden = false; }

  function renderCompressResult(payload) {
    showResultPanel();
    lastCompressPayload = payload;
    const a = payload.analysis || {};
    barsCard.hidden = false; algoCompareCard.hidden = false; codesCard.hidden = false; histCard.hidden = false;
    checksumWarning.hidden = true;

    resultFilename.textContent = payload.filename;
    resultSubtitle.textContent = `Compressed with ${payload.algorithm} in ${payload.processingTimeMs.toFixed(2)} ms`;
    resultSubtitle.classList.remove("warn");

    statOriginal.textContent = formatBytes(payload.originalSize);
    statCompressed.textContent = formatBytes(payload.compressedSize);
    statSaved.textContent = `${formatBytes(payload.spaceSavedBytes)} (${formatPercent(payload.spaceSavedPercent)})`;
    statAlgo.textContent = payload.algorithm;
    statEntropy.textContent = a.entropyBitsPerByte != null ? `${a.entropyBitsPerByte.toFixed(2)} b/B` : "—";
    // Efficiency: how close compressed size is to the entropy lower bound.
    if (a.theoreticalMinBytes && payload.compressedSize) {
      const eff = Math.min(100, (a.theoreticalMinBytes / payload.compressedSize) * 100);
      statEfficiency.textContent = `${eff.toFixed(0)}%`;
    } else statEfficiency.textContent = "—";
    statTime.textContent = `${payload.processingTimeMs.toFixed(2)} ms`;
    statChecksum.textContent = payload.checksum;

    // Size bars (original / compressed / theoretical min)
    const theo = a.theoreticalMinBytes || 0;
    const maxSize = Math.max(payload.originalSize, payload.compressedSize, theo, 1);
    requestAnimationFrame(() => {
      barOriginal.style.width = `${(payload.originalSize / maxSize) * 100}%`;
      barCompressed.style.width = `${(payload.compressedSize / maxSize) * 100}%`;
      barTheoretical.style.width = `${(theo / maxSize) * 100}%`;
    });
    barOriginalValue.textContent = formatBytes(payload.originalSize);
    barCompressedValue.textContent = formatBytes(payload.compressedSize);
    barTheoreticalValue.textContent = formatBytes(theo);

    renderAlgoCompare(a.comparisons || []);
    renderHistogram(a.histogram || []);
    drawTree(a.tree, a.distinctSymbols);
    renderCodeTable(a.histogram || []);

    currentDownload = { base64: payload.fileBase64, filename: payload.downloadName };
  }

  function renderDecompressResult(payload) {
    showResultPanel();
    lastCompressPayload = null;
    barsCard.hidden = true; algoCompareCard.hidden = true; codesCard.hidden = true; histCard.hidden = true; treeCard.hidden = true;

    resultFilename.textContent = payload.downloadName;
    if (payload.checksumVerified) {
      resultSubtitle.textContent = `Restored via ${payload.algorithm} in ${payload.processingTimeMs.toFixed(2)} ms — integrity verified`;
      resultSubtitle.classList.remove("warn");
      checksumWarning.hidden = true;
    } else {
      resultSubtitle.textContent = `Restored in ${payload.processingTimeMs.toFixed(2)} ms — checksum mismatch`;
      resultSubtitle.classList.add("warn");
      checksumWarning.hidden = false;
    }

    statOriginal.textContent = formatBytes(payload.decompressedSize);
    statCompressed.textContent = "—";
    statSaved.textContent = "—";
    statAlgo.textContent = payload.algorithm;
    statEntropy.textContent = "—";
    statEfficiency.textContent = "—";
    statTime.textContent = `${payload.processingTimeMs.toFixed(2)} ms`;
    statChecksum.textContent = payload.checksum;

    currentDownload = { base64: payload.fileBase64, filename: payload.downloadName };
  }

  // ---------------------------------------------------------------
  // Algorithm comparison bars
  // ---------------------------------------------------------------
  function renderAlgoCompare(comparisons) {
    algoCompare.innerHTML = "";
    if (!comparisons.length) { algoCompareCard.hidden = true; return; }
    const maxSize = Math.max(...comparisons.map((c) => c.size), 1);
    comparisons.forEach((c) => {
      const savedClass = c.spaceSavedPercent >= 0 ? "savings-good" : "savings-bad";
      const row = document.createElement("div");
      row.className = "ac-row";
      row.innerHTML = `
        <div class="ac-name">${c.best ? '<span class="star">★</span>' : ""}${escapeHtml(c.name)}</div>
        <div class="ac-track"><div class="ac-fill ${c.best ? "best" : ""}"></div></div>
        <div class="ac-value">${formatBytes(c.size)} <span class="${savedClass}">(${formatPercent(c.spaceSavedPercent)})</span></div>`;
      algoCompare.appendChild(row);
      const fill = row.querySelector(".ac-fill");
      requestAnimationFrame(() => { fill.style.width = `${(c.size / maxSize) * 100}%`; });
    });
  }

  // ---------------------------------------------------------------
  // Histogram
  // ---------------------------------------------------------------
  function renderHistogram(hist) {
    histogramEl.innerHTML = "";
    if (!hist.length) { histCard.hidden = true; return; }
    const max = Math.max(...hist.map((h) => h.frequency), 1);
    hist.slice(0, 48).forEach((h) => {
      const bar = document.createElement("div");
      bar.className = "hist-bar";
      bar.style.height = `${Math.max(3, (h.frequency / max) * 100)}%`;
      bar.innerHTML = `<span class="hist-tip">'${escapeHtml(h.display)}' × ${h.frequency.toLocaleString()} (${h.percent.toFixed(1)}%)</span>`;
      histogramEl.appendChild(bar);
    });
  }

  // ---------------------------------------------------------------
  // Code table
  // ---------------------------------------------------------------
  function renderCodeTable(hist) {
    currentCodes = hist;
    codesTbody.innerHTML = "";
    hist.forEach((h) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${escapeHtml(h.display)}</td>
        <td>${h.frequency.toLocaleString()}</td>
        <td>${h.percent.toFixed(2)}%</td>
        <td>${h.code || "—"}</td>
        <td>${h.codeLength || "—"}</td>`;
      codesTbody.appendChild(tr);
    });
  }

  // ---------------------------------------------------------------
  // Huffman tree SVG visualizer
  // ---------------------------------------------------------------
  function drawTree(tree, distinctSymbols) {
    if (!tree) {
      treeCard.hidden = true;
      return;
    }
    treeCard.hidden = false;
    treeHint.textContent = `${distinctSymbols || countLeaves(tree)} symbols`;

    // Assign x/y via an in-order layout for leaves, then place parents at
    // the midpoint of their children. Depth drives the y coordinate.
    const levelGap = 62;
    const leafGap = 54;
    let leafIndex = 0;
    let maxDepth = 0;

    function layout(node, depth) {
      maxDepth = Math.max(maxDepth, depth);
      if (node.isLeaf || (!node.left && !node.right)) {
        node._x = leafIndex * leafGap + leafGap / 2;
        node._y = depth * levelGap + 30;
        leafIndex++;
        return;
      }
      if (node.left) layout(node.left, depth + 1);
      if (node.right) layout(node.right, depth + 1);
      const lx = node.left ? node.left._x : 0;
      const rx = node.right ? node.right._x : 0;
      node._x = (lx + rx) / 2;
      node._y = depth * levelGap + 30;
    }
    layout(tree, 0);

    const width = Math.max(leafIndex * leafGap + leafGap, 300);
    const height = (maxDepth + 1) * levelGap + 40;

    const parts = [`<svg viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" xmlns="http://www.w3.org/2000/svg">`];

    // Edges first (with 0/1 labels).
    function edges(node) {
      if (!node) return;
      [["left", "0"], ["right", "1"]].forEach(([side, bit]) => {
        const child = node[side];
        if (!child) return;
        const mx = (node._x + child._x) / 2;
        const my = (node._y + child._y) / 2;
        parts.push(`<line class="tree-edge" x1="${node._x}" y1="${node._y}" x2="${child._x}" y2="${child._y}" />`);
        parts.push(`<text class="tree-edge-label" x="${mx + (bit === "0" ? -8 : 6)}" y="${my}">${bit}</text>`);
        edges(child);
      });
    }
    edges(tree);

    // Nodes on top.
    function nodes(node) {
      if (!node) return;
      const isLeaf = node.isLeaf || (!node.left && !node.right);
      if (isLeaf) {
        parts.push(`<circle class="tree-node-leaf" cx="${node._x}" cy="${node._y}" r="15" />`);
        parts.push(`<text class="tree-leaf-label" x="${node._x}" y="${node._y + 4}">${escapeXml(node.display || "?")}</text>`);
        parts.push(`<text class="tree-freq-label" x="${node._x}" y="${node._y + 27}">${node.freq}</text>`);
      } else {
        parts.push(`<circle class="tree-node-internal" cx="${node._x}" cy="${node._y}" r="11" />`);
        parts.push(`<text class="tree-freq-label" x="${node._x}" y="${node._y - 16}">${node.freq}</text>`);
        nodes(node.left); nodes(node.right);
      }
    }
    nodes(tree);

    parts.push("</svg>");
    treeHolder.innerHTML = parts.join("");
  }

  function countLeaves(node) {
    if (!node) return 0;
    if (node.isLeaf || (!node.left && !node.right)) return 1;
    return countLeaves(node.left) + countLeaves(node.right);
  }

  // ---------------------------------------------------------------
  // Download / copy
  // ---------------------------------------------------------------
  downloadBtn.addEventListener("click", () => { if (currentDownload) triggerDownload(currentDownload.base64, currentDownload.filename); });
  copyCodesBtn.addEventListener("click", async () => {
    if (!currentCodes) return;
    try { await navigator.clipboard.writeText(JSON.stringify(currentCodes, null, 2)); showToast("success", "Code table copied to clipboard"); }
    catch (e) { showToast("error", "Could not copy to clipboard"); }
  });
  function triggerDownload(base64, filename) {
    const bytes = base64ToBytes(base64);
    const blob = new Blob([bytes], { type: "application/octet-stream" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url; a.download = filename;
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 4000);
  }
  function base64ToBytes(base64) {
    const bin = atob(base64);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return bytes;
  }

  // ---------------------------------------------------------------
  // History
  // ---------------------------------------------------------------
  const historyList = document.getElementById("history-list");
  const clearHistoryBtn = document.getElementById("clear-history");
  function addHistory(entry) { entry.id = ++historySeq; entry.time = new Date(); history.unshift(entry); renderHistory(); }
  function renderHistory() {
    historyList.innerHTML = "";
    if (!history.length) { historyList.innerHTML = '<li class="empty-state">No operations yet this session.</li>'; return; }
    history.forEach((entry) => {
      const li = document.createElement("li");
      li.className = "history-item" + (entry.kind === "error" ? " err" : "");
      const icon = entry.kind === "compress" ? "📦" : entry.kind === "decompress" ? "📂" : "⚠️";
      const meta = entry.kind === "error" ? entry.error
        : entry.kind === "compress" ? `${formatBytes(entry.originalSize)} → ${formatBytes(entry.resultSize)}`
        : `restored · ${formatBytes(entry.resultSize)}`;
      li.innerHTML = `<span class="hi-icon">${icon}</span><div style="flex:1; min-width:0;"><div class="hi-name">${escapeHtml(entry.name)}</div><div class="hi-meta">${escapeHtml(meta)} · ${formatTime(entry.time)}</div></div>`;
      if (entry.kind !== "error") li.addEventListener("click", () => { entry.kind === "compress" ? renderCompressResult(entry.payload) : renderDecompressResult(entry.payload); });
      historyList.appendChild(li);
    });
  }
  clearHistoryBtn.addEventListener("click", () => { history.length = 0; renderHistory(); });

  // ---------------------------------------------------------------
  // Toasts + helpers
  // ---------------------------------------------------------------
  const toastContainer = document.getElementById("toast-container");
  const toastIcons = { success: "✅", error: "❌", info: "ℹ️" };
  function showToast(kind, message) {
    const el = document.createElement("div");
    el.className = `toast ${kind}`;
    el.innerHTML = `<span>${toastIcons[kind] || ""}</span><span>${escapeHtml(message)}</span>`;
    toastContainer.appendChild(el);
    setTimeout(() => { el.style.opacity = "0"; el.style.transform = "translateX(20px)"; el.style.transition = "all 0.25s ease"; setTimeout(() => el.remove(), 260); }, 4200);
  }

  function formatBytes(n) {
    if (n === 0) return "0 B";
    const neg = n < 0; n = Math.abs(n);
    const units = ["B", "KB", "MB", "GB"];
    let i = 0, val = n;
    while (val >= 1024 && i < units.length - 1) { val /= 1024; i++; }
    return `${neg ? "-" : ""}${val.toFixed(val < 10 && i > 0 ? 2 : 0)} ${units[i]}`;
  }
  function formatPercent(p) { return `${p.toFixed(1)}%`; }
  function formatTime(d) { return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }); }
  function escapeHtml(str) { const div = document.createElement("div"); div.textContent = str; return div.innerHTML; }
  function escapeXml(str) { return String(str).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&apos;"); }

  fetch("/api/health").catch(() => {});
})();

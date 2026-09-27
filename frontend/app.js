// TipReader frontend — native JS. Talks to Go via the Wails v3 runtime.
import { Call } from "/wails/runtime.js";

(function () {
    "use strict";

    const $ = (id) => document.getElementById(id);

    const tele = $("teleprompter");
    const linesEl = $("lines");
    const overlay = $("role-overlay");
    const notice = $("notice");
    const status = $("status");

    let state = null;
    let role = "01";
    let lineHeight = 1.15;
    let lastHash = "";

    // --- Wails v3 runtime bridge ---
    async function call(method, ...args) {
        return await Call.ByName("main.App." + method, ...args);
    }

    // --- rendering ---
    function applyStyle(settings) {
        const size = settings.fontSize || 32;
        const lh = lineHeight;
        const rows = linesEl.children;
        for (const row of rows) {
            if (row.classList.contains("blank")) {
                row.style.height = Math.round(size * lh * 0.6) + "px";
                continue;
            }
            row.style.fontSize = size + "px";
            row.style.lineHeight = lh + "";
        }
    }

    function topLine() {
        return Math.round(tele.clientHeight * (state?.settings?.centerRatio || 0.45));
    }

    function centerCursor() {
        if (!state) return;
        const target = tele.querySelector('.script-line[data-id="' + state.cursor + '"]');
        if (!target) return;
        const offset = target.offsetTop - topLine() + target.offsetHeight / 2 + 8;
        tele.scrollTop = Math.max(0, offset);
    }

    function render(s) {
        state = s || state;
        if (!state) return;

        const settings = state.settings || {};
        role = settings.role || "01";

        // role classes
        tele.parentElement.classList.toggle("controller", role === "01");

        const textColor = settings.textColor || "#FFFFFF";
        const readColor = settings.readColor || "#FF0000";

        // build lines
        const frag = document.createDocumentFragment();
        for (const line of (state.lines || [])) {
            const div = document.createElement("div");
            if (line.id < 0) {
                div.className = "script-line blank";
                div.dataset.id = "-1";
            } else {
                div.className = "script-line";
                div.dataset.id = String(line.id);
                div.textContent = line.text;
                // read = current and all previous lines turn read color (spec §20-21)
                const isRead = line.id <= state.cursor;
                const isCurrent = line.id === state.cursor;
                if (isRead || isCurrent) {
                    div.classList.add(isCurrent ? "current" : "read");
                    div.style.color = readColor;
                } else {
                    div.style.color = textColor;
                }
                if (role === "01") div.classList.add("clickable");
            }
            frag.appendChild(div);
        }
        // re-render only if changed to avoid disrupting selection
        const hash = JSON.stringify({ ids: (state.lines || []).map(l => l.id), c: state.cursor, r: role });
        if (lastHash === hash) {
            applyStyle(settings);
            return;
        }
        lastHash = hash;
        linesEl.innerHTML = "";
        linesEl.appendChild(frag);
        applyStyle(settings);
        centerCursor();

        // notice / status
        const hasContent = (state.lines || []).some(l => l.id >= 0) ;
        status.textContent = "";
        if (!hasContent) {
            showNotice("未找到 read.md\n请将 read.md 放到 TipReader.exe 同一文件夹。");
        } else {
            hideNotice();
        }
    }

    function showNotice(msg) { if (msg) { notice.textContent = msg; notice.classList.remove("hidden"); } }
    function hideNotice() { notice.classList.add("hidden"); }

    // --- interactions ---
    function refresh() {
        call("PullState").then((s) => {
            window.__remoteErr = (s && s.error) || "";
            render(s);
        }).catch(() => { setTimeout(refresh, 800); });
    }

    linesEl.addEventListener("click", (e) => {
        if (role !== "01") return; // 被控端禁止点击操作
        const row = e.target.closest(".script-line");
        if (!row) return;
        const id = parseInt(row.dataset.id, 10);
        if (id < 0) return;
        call("SetCursor", id).then(render);
    });

    $("btn-next").addEventListener("click", () => call("CursorNext").then(render));
    $("btn-prev").addEventListener("click", () => call("CursorPrev").then(render));
    $("btn-reload").addEventListener("click", () => call("Reload").then(render));
    $("btn-close").addEventListener("click", () => call("CloseApp"));

    // settings panel
    const panel = $("settings");
    $("btn-settings").addEventListener("click", () => {
        panel.classList.toggle("open");
        if (panel.classList.contains("open") && state?.settings) fillPanel(state.settings);
    });
    $("close-settings").addEventListener("click", () => panel.classList.remove("open"));
    $("apply-settings").addEventListener("click", () => applyPanel());

    function fillPanel(s) {
        $("set-fontsize").value = s.fontSize || 32;
        $("set-textcolor").value = s.textColor || "#FFFFFF";
        $("set-readcolor").value = s.readColor || "#FF0000";
        $("set-top").checked = !!s.alwaysOnTop;
        $("set-port").value = s.port || 7777;
        $("set-ip").value = s.controllerIP || "127.0.0.1";
        $("set-cpip").value = s.controllerPort || 7777;
    }
    function applyPanel() {
        const s = (state && state.settings) || {};
        const next = {
            ...s,
            fontSize: parseInt($("set-fontsize").value, 10) || 32,
            textColor: $("set-textcolor").value,
            readColor: $("set-readcolor").value,
            alwaysOnTop: $("set-top").checked,
            port: parseInt($("set-port").value, 10) || 7777,
            controllerIP: $("set-ip").value,
            controllerPort: parseInt($("set-cpip").value, 10) || 7777,
        };
        lineHeight = parseFloat($("set-lineheight").value || "1.3");
        call("SaveSettings", next).then((s) => { render({ ...state, settings: s }); panel.classList.remove("open"); });
    }
    $("set-lineheight").addEventListener("input", (e) => {
        lineHeight = parseFloat(e.target.value);
        if (state?.lines?.length) applyStyle(state.settings || {});
    });

    // 窗口拖动由 .drag-area 的 --wails-draggable: drag 交给 Wails v3 运行时处理，
    // 无需自定义 JS 拖拽逻辑。

    // --- keyboard (spec §52-54) ---
    window.addEventListener("keydown", (e) => {
        if (role === "02") {
            // 被控端阻止用户滚动/改行
            const keys = ["ArrowDown", "ArrowUp", "PageDown", "PageUp", "Home", "End", " "];
            if (keys.includes(e.key)) e.preventDefault();
            return;
        }
        if (e.ctrlKey && (e.key === "r" || e.key === "R")) { e.preventDefault(); call("Reload").then(render); return; }
        if (e.key === "Escape") { call("CloseApp"); return; }
        if (e.key === "ArrowDown" || e.key === " ") { e.preventDefault(); call("CursorNext").then(render); return; }
        if (e.key === "ArrowUp") { e.preventDefault(); call("CursorPrev").then(render); }
    });

    // viewer: block wheel scroll; controller: allow wheel but recenter is fine
    tele.addEventListener("wheel", (e) => {
        if (role === "02") { e.preventDefault(); return; }
        if (role === "01") {
            // 控制端滚轮可自由滚动（规格 §53）
            return;
        }
    }, { passive: false });

    // recenter on window resize
    window.addEventListener("resize", () => centerCursor());

    // polling keeps viewer (and window) in sync over the network in real time
    setInterval(refresh, 700);

    // --- boot: 打开直接显示稿件，无角色选择 ---
    (async function boot() {
        try {
            state = await call("GetScript");
        } catch (e) { console.error(e); }
        if (state?.settings) role = "01";
        overlay.classList.add("hidden");
        refresh();
    })();
})();
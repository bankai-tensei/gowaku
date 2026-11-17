// The website of gowaku: the theme switch, copy buttons, the side bar on
// narrow screens, the table of contents, and the search of the catalog.
(function () {
  "use strict";
  var root = document.documentElement;
  var words = window.GOWAKU_WORDS || { copy: "Copy", copied: "Copied" };

  function store(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }

  // Theme: follow the OS until the button is pressed, then remember.
  var theme = document.getElementById("theme");
  if (theme) {
    theme.addEventListener("click", function () {
      var dark = root.dataset.theme ? root.dataset.theme === "dark" : matchMedia("(prefers-color-scheme: dark)").matches;
      root.dataset.theme = dark ? "light" : "dark";
      store("gowaku-theme", root.dataset.theme);
    });
  }

  function copy(text, el) {
    var done = function () {
      el.classList.add("done");
      el.setAttribute("aria-label", words.copied);
      setTimeout(function () { el.classList.remove("done"); el.setAttribute("aria-label", words.copy); }, 1400);
    };
    if (navigator.clipboard) navigator.clipboard.writeText(text).then(done, function () {});
  }
  document.querySelectorAll(".code .copy").forEach(function (b) {
    b.addEventListener("click", function () { copy(b.parentNode.querySelector("pre").innerText, b); });
  });
  document.querySelectorAll("[data-copy]").forEach(function (b) {
    b.addEventListener("click", function () { copy(b.dataset.copy, b); });
  });

  // The side bar slides in on narrow screens.
  var toggle = document.querySelector(".side-toggle"), side = document.getElementById("side");
  if (toggle && side) {
    toggle.addEventListener("click", function () {
      var open = side.classList.toggle("open");
      toggle.setAttribute("aria-expanded", open);
    });
    side.addEventListener("click", function (e) { if (e.target.tagName === "A") side.classList.remove("open"); });
    var on = side.querySelector("a.on");
    if (on && on.scrollIntoView) on.scrollIntoView({ block: "center" });
  }

  // The table of contents marks the section being read.
  var toc = document.querySelectorAll(".toc a");
  if (toc.length && "IntersectionObserver" in window) {
    var map = {};
    toc.forEach(function (a) { map[decodeURIComponent(a.hash.slice(1))] = a; });
    var current;
    var io = new IntersectionObserver(function (es) {
      es.forEach(function (e) {
        if (e.isIntersecting && map[e.target.id]) {
          if (current) current.classList.remove("on");
          current = map[e.target.id];
          current.classList.add("on");
        }
      });
    }, { rootMargin: "-70px 0px -70% 0px" });
    Object.keys(map).forEach(function (id) { var el = document.getElementById(id); if (el) io.observe(el); });
  }

  // The catalog: search and category chips filter the cards.
  var q = document.getElementById("q"), grid = document.getElementById("grid");
  if (q && grid) {
    var cards = grid.querySelectorAll(".card"), chips = document.querySelectorAll(".chip"), empty = document.getElementById("empty");
    var cat = "";
    var apply = function () {
      var words = q.value.toLowerCase().split(/\s+/).filter(Boolean), shown = 0;
      cards.forEach(function (c) {
        var ok = (!cat || c.dataset.cat === cat) && words.every(function (w) { return c.dataset.search.indexOf(w) >= 0; });
        c.hidden = !ok;
        if (ok) shown++;
      });
      empty.hidden = shown > 0;
    };
    q.addEventListener("input", apply);
    chips.forEach(function (ch) {
      ch.addEventListener("click", function () {
        chips.forEach(function (x) { x.classList.toggle("on", x === ch); });
        cat = ch.dataset.cat;
        apply();
      });
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "/" && document.activeElement !== q) { e.preventDefault(); q.focus(); }
      if (e.key === "Escape" && document.activeElement === q) { q.value = ""; apply(); q.blur(); }
    });
  }
})();
document.querySelectorAll(".compare input").forEach(function (r) {
  r.addEventListener("input", function () { r.parentNode.style.setProperty("--x", r.value + "%"); });
});

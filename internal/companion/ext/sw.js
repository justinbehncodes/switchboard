// Switchboard Companion: when a page opens a link in a new tab (the pattern
// for Google Chat, Gmail, and most web apps), ask switchboard.exe whether the
// URL belongs in a different browser profile. If it does, Switchboard opens
// it there and we close the tab that was spawned here.
//
// Google Chat and Gmail often wrap links in a google.com/url redirector, so
// we unwrap known wrappers immediately AND keep watching the spawned tab's
// first few navigations — that way server-side redirect chains still get
// checked against the real destination.
//
// Navigations typed into the address bar or opened by Switchboard itself have
// no source tab, so nothing here fires for them — no loops.

const HOST = "com.switchboard.router";

const isEdge = (navigator.userAgentData?.brands || [])
  .some(b => b.brand.includes("Edge"));
const BROWSER = isEdge ? "edge" : "chrome";

// tabId -> {from, until, hops, sent, grabbed} for tabs spawned by a page
// click. `sent` dedupes URLs: the initial navigation surfaces through BOTH
// onCreatedNavigationTarget and onCommitted, which would otherwise launch
// the target browser twice.
const tracked = new Map();

function unwrap(u) {
  try {
    const url = new URL(u);
    const host = url.hostname.toLowerCase();
    if ((host === "www.google.com" || host === "google.com") && url.pathname === "/url") {
      const t = url.searchParams.get("q") || url.searchParams.get("url");
      if (t && /^https?:\/\//i.test(t)) return t;
    }
    if (host.endsWith(".safelinks.protection.outlook.com")) {
      const t = url.searchParams.get("url");
      if (t && /^https?:\/\//i.test(t)) return t;
    }
  } catch (e) { /* fall through */ }
  return u;
}

async function check(tabId, rawUrl, from) {
  const url = unwrap(rawUrl);
  if (!/^https?:\/\//i.test(url)) return;

  // Mark the URL as sent BEFORE any await so the near-simultaneous
  // created/committed events can never both dispatch it.
  const t = tracked.get(tabId);
  if (t) {
    if (t.grabbed || t.sent.has(url)) return;
    t.sent.add(url);
  }

  let email = "";
  try {
    const info = await chrome.identity.getProfileUserInfo({ accountStatus: "ANY" });
    email = info?.email || "";
  } catch (e) { /* identity unavailable */ }

  chrome.runtime.sendNativeMessage(
    HOST,
    { url, from, browser: BROWSER, email },
    resp => {
      if (chrome.runtime.lastError) return; // host not installed: stay put
      if (resp?.action === "grabbed") {
        const cur = tracked.get(tabId);
        if (cur) cur.grabbed = true;
        chrome.tabs.remove(tabId).catch(() => {});
      }
    }
  );
}

chrome.webNavigation.onCreatedNavigationTarget.addListener(async details => {
  let from = "";
  try {
    const tab = await chrome.tabs.get(details.sourceTabId);
    if (tab?.url) from = new URL(tab.url).hostname;
  } catch (e) { /* source tab already gone */ }

  tracked.set(details.tabId, {
    from, until: Date.now() + 15000, hops: 0, sent: new Set(), grabbed: false,
  });
  check(details.tabId, details.url, from);
});

// Follow the spawned tab through redirects (google.com/url -> real target).
chrome.webNavigation.onCommitted.addListener(details => {
  if (details.frameId !== 0) return;
  const t = tracked.get(details.tabId);
  if (!t || t.grabbed) return;
  if (Date.now() > t.until || ++t.hops > 4) {
    tracked.delete(details.tabId);
    return;
  }
  check(details.tabId, details.url, t.from);
});

chrome.tabs.onRemoved.addListener(tabId => tracked.delete(tabId));

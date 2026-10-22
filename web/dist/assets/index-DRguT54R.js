(function(){const o=document.createElement("link").relList;if(o&&o.supports&&o.supports("modulepreload"))return;for(const e of document.querySelectorAll('link[rel="modulepreload"]'))i(e);new MutationObserver(e=>{for(const t of e)if(t.type==="childList")for(const c of t.addedNodes)c.tagName==="LINK"&&c.rel==="modulepreload"&&i(c)}).observe(document,{childList:!0,subtree:!0});function n(e){const t={};return e.integrity&&(t.integrity=e.integrity),e.referrerPolicy&&(t.referrerPolicy=e.referrerPolicy),e.crossOrigin==="use-credentials"?t.credentials="include":e.crossOrigin==="anonymous"?t.credentials="omit":t.credentials="same-origin",t}function i(e){if(e.ep)return;e.ep=!0;const t=n(e);fetch(e.href,t)}})();const s=document.querySelector("#app");async function u(){const r=await fetch("/api/health"),o=await r.json();return{ok:r.ok,body:o}}s.innerHTML=`
  <main>
    <h1>deploy-lab</h1>
    <p class="hint">一个用来练部署的最小全栈项目。</p>
    <button id="check">检查后端健康</button>
    <pre id="out">点击按钮请求 /api/health</pre>
  </main>
`;document.querySelector("#check").addEventListener("click",async()=>{const r=document.querySelector("#out");r.textContent="请求中…";try{const{ok:o,body:n}=await u();r.textContent=(o?`健康
`:`异常
`)+JSON.stringify(n,null,2)}catch(o){r.textContent="请求失败："+o.message}});

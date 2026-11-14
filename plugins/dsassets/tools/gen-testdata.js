#!/usr/bin/env node
// 生成 testdata/official/：每条素材的**官方 JSX 原文片段** + manifest.json（sha256 清单）。
//
//   node plugins/dsassets/tools/extract-official.js   # 先解包到 tmp/official/
//   node plugins/dsassets/tools/gen-testdata.js       # 再生成 testdata
//
// 只读 tmp/official/（官方包解包结果），只写 plugins/dsassets/testdata/official/。
// 生成完直接跑 `go test ./plugins/dsassets/`：常量与原文对不上会当场红。
const fs = require('fs');
const path = require('path');

const REPO = path.resolve(__dirname, '..', '..', '..');
const OFF = path.join(REPO, 'tmp', 'official');
const OUT = path.join(REPO, 'plugins', 'dsassets', 'testdata', 'official');

const WEB = 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js';
const CONV = 'dsh-client-ui-conversation/lib/client.js';

const webSrc = fs.readFileSync(path.join(OFF, WEB), 'utf8');
const convSrc = fs.readFileSync(path.join(OFF, CONV), 'utf8');

// 取 `function NAME({size...}){...}` 的完整函数体
function fnBody(src, name) {
  // 两种定义：`function Ay({size...}){...}` 与 `Ay=({size...})=>d.jsxs(...)`。
  let i = src.indexOf('function ' + name + '({size');
  if (i < 0) {
    i = src.indexOf(name + '=({size');
    if (i < 0) throw new Error('missing component ' + name);
  }
  let depth = 0;
  let j = src.indexOf('"svg"', i) >= 0 ? src.indexOf('{', src.indexOf('"svg"', i)) : src.indexOf('{', i);
  const start = j;
  for (; j < src.length; j++) {
    if (src[j] === '{') depth++;
    else if (src[j] === '}') {
      depth--;
      if (depth === 0) break;
    }
  }
  return src.slice(start, j + 1);
}

// 从函数体里切出所有图元调用原文：d.jsx("path",{...}) / d.jsx("rect",{...})
function elements(src) {
  const out = [];
  const re = /d\.jsx[s]?\("(path|rect|circle|ellipse|line|polygon|polyline)",\s*\{/g;
  let m;
  while ((m = re.exec(src))) {
    const start = m.index;
    let depth = 0;
    let i = src.indexOf('{', start);
    for (; i < src.length; i++) {
      if (src[i] === '{') depth++;
      else if (src[i] === '}') {
        depth--;
        if (depth === 0) break;
      }
    }
    out.push(src.slice(start, i + 2)); // 含结尾 "})"
  }
  return out;
}

function attrs(fragment) {
  const o = {};
  const re = /([A-Za-z-]+):\s*"([^"]*)"/g;
  let m;
  while ((m = re.exec(fragment))) o[m[1]] = m[2];
  return o;
}

const sha256 = (s) => require('crypto').createHash('sha256').update(s).digest('hex');

// 顺着官方导出表找组件本体：导出名 → 绑定（如 `VT`）→ 包装组件
// （`VT=e=>d.jsx(Ev,{...e,strokeWidth:K})`）→ 内层组件（如 `Ev`）。
//
// 为什么不用正则一次搞定：包装体里的 `=>`、`{...e,strokeWidth:K}` 在压缩产物里
// 没有空格，写成一条正则很容易在某次官方升级后"看着像匹配上了其实没匹配"。
// 这里每一步都显式检查，找不到就抛错——比静默生成一个空片段好。
function iconByExport(src, name) {
  const m = new RegExp(name + ':([A-Za-z0-9_$]+)').exec(src);
  if (!m) throw new Error('missing export ' + name);
  const bind = m[1];
  const at = src.indexOf(bind + '=e=>d.jsx(');
  if (at < 0) throw new Error('missing wrapper ' + bind);
  const open = src.indexOf('d.jsx(', at) + 'd.jsx('.length;
  const comma = src.indexOf(',', open);
  if (comma < 0) throw new Error('missing inner component for ' + bind);
  return { binding: bind, inner: src.slice(open, comma) };
}

// 加号（官方 InputBar 命令入口 `.GRgSLq_add` 用的 `<IconPlusOutlineMedium>`）。
// 它是 **Medium** 档：内层组件本体与 Regular 共用，只有 strokeWidth 换成 `K=1.3`。
const plus = iconByExport(webSrc, 'IconPlusOutlineMedium');

// 输入框行内发送/停止：从 conversation 里按锚点切
function composerInline() {
  const sendAnchor = 'd: "M8.3125 0.980183';
  const i = convSrc.indexOf(sendAnchor);
  if (i < 0) throw new Error('composer send path not found');
  const start = convSrc.lastIndexOf('(0, react_jsx_runtime.jsx)("path", {', i);
  const end = convSrc.indexOf('})', i) + 2;
  const send = convSrc.slice(start, end);

  const rectAnchor = 'rx: "3",\n\t\t\t\t\t\t\t\t\t\t\t\t\t\tfill: "currentColor"';
  const j = convSrc.indexOf(rectAnchor);
  if (j < 0) throw new Error('composer stop rect not found');
  const rstart = convSrc.lastIndexOf('(0, react_jsx_runtime.jsx)("rect", {', j);
  const rend = convSrc.indexOf('})', j) + 2;
  const stop = convSrc.slice(rstart, rend);
  return { send, stop };
}

const inline = composerInline();

const ASSETS = [
  { id: 'composer-send', pkg: '@deepseek-ai/dsh-client-ui-conversation', file: 'dsh-client-ui-conversation/lib/client.js', src: CONV,
    symbol: 'InputBar 主按钮内联 path', frags: [inline.send] },
  { id: 'composer-stop', pkg: '@deepseek-ai/dsh-client-ui-conversation', file: 'dsh-client-ui-conversation/lib/client.js', src: CONV,
    symbol: 'InputBar 主按钮内联 rect', frags: [inline.stop] },
  { id: 'send-outline', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconSendOutlineRegular', frags: elements(fnBody(webSrc, 'Ay')) },
  { id: 'paperclip', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconPaperclipOutlineRegular', frags: elements(fnBody(webSrc, 'gy')) },
  { id: 'composer-add', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconPlusOutlineMedium', binding: plus.binding, frags: elements(fnBody(webSrc, plus.inner)) },
  { id: 'copy', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconCopyOutlineRegular', frags: elements(fnBody(webSrc, 'Wv')) },
  { id: 'branch', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconBranchOutlineRegular', frags: elements(fnBody(webSrc, 'Lv')) },
  { id: 'thumb-up-outline', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconLikeOutlineRegular', frags: elements(fnBody(webSrc, 'qv')) },
  { id: 'thumb-up-fill', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconLikeFillRegular', frags: elements(fnBody(webSrc, 'Kv')) },
  { id: 'thumb-down-outline', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconDislikeOutlineRegular', frags: elements(fnBody(webSrc, 'Gv')) },
  { id: 'thumb-down-fill', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'IconDislikeFillRegular', frags: elements(fnBody(webSrc, 'Yv')) },
  { id: 'permission-read-only', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'PermissionIconReadOnlyRegular', binding: 'z_', frags: elements(fnBody(webSrc, 'z_')) },
  { id: 'permission-workspace-write', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'PermissionIconWorkspaceWriteRegular', binding: 'E_', frags: elements(fnBody(webSrc, 'E_')) },
  { id: 'permission-full-access', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'PermissionIconFullAccessRegular', binding: 'O_', frags: elements(fnBody(webSrc, 'O_')) },
  { id: 'permission-auto-review', pkg: '@deepseek-ai/dsh-client-ui-primitives', file: 'dsh-web-frontend/dist/assets/index-BXV-oxvX.js', src: WEB,
    symbol: 'PermissionIconAutoReviewRegular', binding: 'M_', frags: elements(fnBody(webSrc, 'M_')) },
];

fs.mkdirSync(OUT, { recursive: true });
const manifest = {};
for (const a of ASSETS) {
  const text = a.frags.join('\n');
  if (/dsh-client-ui-primitives/.test(a.pkg) && a.symbol.includes('(') ) { /* 已核对 */ }
  fs.writeFileSync(path.join(OUT, a.id + '.jsx'), text + '\n');
  const dv = [];
  for (const f of a.frags) {
    const at = attrs(f);
    if (at.d) dv.push(at.d);
  }
  manifest[a.id] = {
    package: a.pkg,
    file: a.file,
    symbol: a.symbol,
    binding: a.binding || null,
    file_sha256: sha256(fs.readFileSync(path.join(OFF, a.src))),
    fragment_sha256: sha256(text),
    paths: dv.map((d) => ({ d, sha256: sha256(d), len: d.length })),
    elements: a.frags.map((f) => {
      const at = attrs(f);
      return at.d ? { tag: 'path', fill: at.fill || null, stroke: at.stroke || null } : { tag: /"(\w+)"/.exec(f)[1], ...at };
    }),
  };
}
fs.writeFileSync(path.join(OUT, 'manifest.json'), JSON.stringify(manifest, null, 1) + '\n');
console.log('wrote', ASSETS.length, 'assets to', OUT);
for (const a of ASSETS) {
  console.log(a.id.padEnd(28), a.frags.length, 'elements', a.frags[0].slice(0, 60).replace(/\s+/g, ' '));
}

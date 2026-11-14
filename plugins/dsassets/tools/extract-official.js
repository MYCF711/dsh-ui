#!/usr/bin/env node
// 从安装态 app.asar 里导出官方素材到仓内 `tmp/official/`（gitignored）。
//
//   node plugins/dsassets/tools/extract-official.js [asar路径]
//
// 只读 asar，绝不写 asar；输出目录固定在仓根 `tmp/official/`（与
// `dsassets.ExtractedDir` 一致，golden 测试就是去那儿核对官方文件哈希的）。
//
// 导出两批：
//   1. `@deepseek-ai/dsh-client-ui-*` 全部文件（发送/停止这类行内图元在这些包里）；
//   2. `@deepseek-ai/dsh-web-frontend/**`（primitives 的图标集**不是独立包**，
//      它被压进 web 前端的 dist chunk，所以必须一起导）。
const fs = require('fs');
const path = require('path');

const REPO = path.resolve(__dirname, '..', '..', '..');
const ASAR = process.argv[2] || 'D:/DeepSeek Harness/Desktop/resources/app.asar';
const OUT = path.join(REPO, 'tmp', 'official');

// asar 格式：8 字节 pickle 头 + 4 字节 JSON 长度 + JSON 头，之后是文件数据。
const buf = fs.readFileSync(ASAR);
const jsonSize = buf.readUInt32LE(12);
const header = JSON.parse(buf.slice(16, 16 + jsonSize).toString('utf8'));
const base = 8 + buf.readUInt32LE(4);

const files = {};
(function walk(node, p) {
  for (const k of Object.keys(node.files || {})) {
    const v = node.files[k];
    const np = p + '/' + k;
    if (v.files) walk(v, np);
    else files[np] = v;
  }
})(header, '');

const PREFIXES = [
  '/dsh/node_modules/@deepseek-ai/dsh-client-ui-',
  '/dsh/node_modules/@deepseek-ai/dsh-web-frontend/',
];

let n = 0;
for (const [p, e] of Object.entries(files)) {
  if (!PREFIXES.some((prefix) => p.startsWith(prefix))) continue;
  const rel = p.slice('/dsh/node_modules/@deepseek-ai/'.length);
  const dest = path.join(OUT, rel);
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  const start = base + Number(e.offset);
  fs.writeFileSync(dest, buf.slice(start, start + e.size));
  n++;
}
console.log('extracted', n, 'files ->', OUT);

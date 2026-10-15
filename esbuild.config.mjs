// GUI の JS アセットをビルドする。CSS は Tailwind CLI（package.json の build スクリプト）が生成する。
//   dist/js/app.js       … src/app.ts のバンドル（将来 src/components/* のエントリを追加できる）
//   dist/js/htmx.min.js  … htmx 本体のコピー（CSP script-src 'self' のため同一オリジンから配信する）
import { copyFile, mkdir } from "node:fs/promises";
import * as esbuild from "esbuild";

const staticDir = "internal/gui/web/static";
const outDir = `${staticDir}/dist/js`;
const watch = process.argv.includes("--watch");

await mkdir(outDir, { recursive: true });
await copyFile("node_modules/htmx.org/dist/htmx.min.js", `${outDir}/htmx.min.js`);

const options = {
  entryPoints: { app: `${staticDir}/src/app.ts` },
  outdir: outDir,
  bundle: true,
  format: "esm",
  target: "es2022",
  minify: !watch,
  sourcemap: watch,
  logLevel: "info",
};

if (watch) {
  const ctx = await esbuild.context(options);
  await ctx.watch();
} else {
  await esbuild.build(options);
}

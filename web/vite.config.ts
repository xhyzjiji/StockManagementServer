import { fileURLToPath, URL } from 'node:url';
import { defineConfig, loadEnv } from 'vite';
import vue from '@vitejs/plugin-vue';
import Components from 'unplugin-vue-components/vite';
import { VantResolver } from '@vant/auto-import-resolver';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  // 开发时把 /api 与 /files 代理到 Go 服务，浏览器始终同源，cookie 无需跨域配置
  // In development /api and /files are proxied to the Go server so the browser stays same-origin.
  const target = env.VITE_API_TARGET || 'http://127.0.0.1:8080';
  return {
    plugins: [
      vue(),
      // 按需自动引入 Vant 组件；样式在 main.ts 中整体引入
      // Auto-import Vant components on demand; styles are imported once in main.ts.
      Components({ dirs: [], resolvers: [VantResolver({ importStyle: false })], dts: 'src/components.d.ts' }),
    ],
    resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
    server: {
      host: true,
      port: 5173,
      proxy: {
        '/api': { target },
        '/files': { target },
      },
    },
    build: { outDir: 'dist', chunkSizeWarningLimit: 1000 },
  };
});

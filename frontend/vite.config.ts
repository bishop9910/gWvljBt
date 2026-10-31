import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import wails from "@wailsio/runtime/plugins/vite";

// 开发模式下窗口直接加载 Vite 页面，Go 侧的资源中间件不参与，
// 所以 /media/ 必须转发到 main.go 里的 startDevMediaServer（端口见 mediaPort）。
const MEDIA_PORT = 39217;

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    proxy: {
      "/media": {
        target: `http://127.0.0.1:${MEDIA_PORT}`,
        changeOrigin: false,
      },
    },
  },
  plugins: [vue(), wails("./bindings")],
});

import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiTarget = env.API_PROXY_TARGET || env.VITE_API_BASE_URL || "http://localhost:3001";

  /*
   * 这个代理只服务本地开发。浏览器访问 `/api/...` 时，请求先到 Vite dev server，
   * Vite 再把它转发到 Go 后端。这样前端代码可以一直写相对路径，不需要在每个请求里
   * 硬编码 localhost 端口。真正部署时可以让静态站点和 Go API 共用域名，或者在反向代理
   * 里配置同样的路径转发；这里不提前引入更复杂的部署抽象。
   */
  return {
    server: {
      proxy: {
        "/api": {
          target: apiTarget,
          changeOrigin: true
        },
        "/healthz": {
          target: apiTarget,
          changeOrigin: true
        }
      }
    }
  };
});

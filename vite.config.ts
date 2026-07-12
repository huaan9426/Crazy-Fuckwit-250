import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiTarget = env.API_PROXY_TARGET || env.VITE_API_BASE_URL || "http://localhost:3001";
  const publicBasePath = env.VITE_PUBLIC_BASE_PATH || "/";

  /*
   * 这个代理只服务本地开发。浏览器访问 `/api/...` 时，请求先到 Vite dev server，
   * Vite 再把它转发到 Go 后端。这样前端代码可以一直写相对路径，不需要在每个请求里
   * 硬编码 localhost 端口。真正部署时可以让静态站点和 Go API 共用域名，或者在反向代理
   * 里配置同样的路径转发；这里不提前引入更复杂的部署抽象。
   */
  return {
    /*
     * Vite 的 base 决定生产 HTML 里的脚本、CSS 和图片从哪个 URL 前缀加载。默认值仍是
     * 根路径，所以本地开发和将来使用独立子域名时没有变化；只有部署到现有域名的
     * `/crazy-250/` 目录时，构建命令才传入同名环境变量。API 仍由 VITE_API_BASE_URL
     * 单独控制，两者不写死在源码里，也不会让静态资源路径和后端地址互相冒充。
     */
    base: publicBasePath,
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

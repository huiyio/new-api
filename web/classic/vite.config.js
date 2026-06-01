/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import react from '@vitejs/plugin-react';
import { defineConfig, transformWithEsbuild } from 'vite';
import pkg from '@douyinfe/vite-plugin-semi';
import path from 'path';
import fs from 'fs';
import { pathToFileURL } from 'url';
import { compileString, Logger } from 'sass';
import { semiThemeLoader } from '@douyinfe/vite-plugin-semi/lib/semi-theme-loader.js';
import { codeInspectorPlugin } from 'code-inspector-plugin';
const { vitePluginSemi } = pkg;

// @douyinfe/vite-plugin-semi 的内置 importer 通过
// `scssFilePath.match(/^(\S*\/node_modules\/)/)` 定位 node_modules，
// 当项目路径包含空格（如 "new api前端优化"）时 `\S*` 匹配失败，
// 导致 `~@douyinfe/semi-theme-default/...` 解析到错误目录、构建报
// "Can't find stylesheet to import"。
// 这里用一个 enforce: 'pre' 插件抢先 load 这些 Semi 样式文件，
// 复用官方 themeLoader，但改用基于路径切分（不含空格限制）的 importer。
function semiThemeFix(options = {}) {
  return {
    name: 'vite-plugin-semi-theme-fix',
    enforce: 'pre',
    load(id) {
      const filePath = id.split('?')[0];
      if (
        !/@douyinfe\/semi-(ui|icons|foundation)\/lib\/.+\.css$/.test(filePath)
      ) {
        return null;
      }
      const scssFilePath = filePath.replace(/\.css$/, '.scss');
      const semiLoaderOptions = {
        name:
          typeof options.theme === 'string'
            ? options.theme
            : options.theme?.name,
        cssLayer: options.cssLayer,
        variables: Object.entries(options.variables || {})
          .map(([k, v]) => `${k}: ${v};\n`)
          .join(''),
      };
      const originalScssRaw = fs.readFileSync(scssFilePath, 'utf-8');
      const newScssRaw = semiThemeLoader(originalScssRaw, semiLoaderOptions);
      const nodeModulesIdx = scssFilePath.lastIndexOf('/node_modules/');
      const nodeModulesDir =
        nodeModulesIdx >= 0
          ? scssFilePath.slice(0, nodeModulesIdx + '/node_modules/'.length)
          : path.join(__dirname, 'node_modules/');
      return compileString(newScssRaw, {
        importers: [
          {
            findFileUrl(url) {
              if (url.startsWith('~')) {
                return pathToFileURL(path.join(nodeModulesDir, url.slice(1)));
              }
              const resolved = path.resolve(path.dirname(scssFilePath), url);
              if (fs.existsSync(resolved)) {
                return pathToFileURL(resolved);
              }
              return null;
            },
          },
        ],
        logger: Logger.silent,
      }).css;
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  plugins: [
    semiThemeFix({
      cssLayer: true,
    }),
    codeInspectorPlugin({
      bundler: 'vite',
    }),
    {
      name: 'treat-js-files-as-jsx',
      async transform(code, id) {
        if (!/src\/.*\.js$/.test(id)) {
          return null;
        }

        // Use the exposed transform from vite, instead of directly
        // transforming with esbuild
        return transformWithEsbuild(code, id, {
          loader: 'jsx',
          jsx: 'automatic',
        });
      },
    },
    react(),
    vitePluginSemi({
      cssLayer: true,
    }),
  ],
  optimizeDeps: {
    force: true,
    esbuildOptions: {
      loader: {
        '.js': 'jsx',
        '.json': 'json',
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'react-core': ['react', 'react-dom', 'react-router-dom'],
          'semi-ui': ['@douyinfe/semi-icons', '@douyinfe/semi-ui'],
          tools: ['axios', 'history', 'marked'],
          'react-components': [
            'react-dropzone',
            'react-fireworks',
            'react-telegram-login',
            'react-toastify',
            'react-turnstile',
          ],
          i18n: [
            'i18next',
            'react-i18next',
            'i18next-browser-languagedetector',
          ],
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/mj': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/pg': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
    },
  },
});

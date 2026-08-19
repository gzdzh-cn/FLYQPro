import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path';
// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
  },
  resolve: {
    alias: [
      {
        find: '@',
        replacement: resolve(__dirname, './src'),
      },
      {
        find: '/@',
        replacement: resolve(__dirname, './src'),
      },
      {
        find: '/#',
        replacement: resolve(__dirname, './bindings'),
      },
    ],
    extensions: ['.ts', '.js'],
  },
  css: {
    preprocessorOptions: {
      less: {
        modifyVars: {
          'arcoblue-6': "#165DFF",//
        },
        javascriptEnabled: true,
      },
    },
  },
})

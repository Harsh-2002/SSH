import path from 'path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(() => {
    return {
      base: '/SSH-MCP/',
      server: {
        port: 3000,
        host: '0.0.0.0',
        allowedHosts: ['dev.ctl.qzz.io'],
      },
      plugins: [react()],
      resolve: {
        alias: {
          '@': path.resolve(__dirname, '.'),
        }
      }
    };
});

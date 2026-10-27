

module.exports = {
  apps: [
    {
      name: 'snjsync',
      cwd: __dirname,
      script: './snjsync_linux',
      instances: 1,
      exec_mode: 'fork',
      autorestart: true,
      max_memory_restart: '500M',
      kill_timeout: 5000,
      env: {
        NODE_ENV: 'production',
        CTF_NO_PG_AUTOSTART: '1',
      },
    },
  ],
};
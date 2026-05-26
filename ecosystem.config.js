module.exports = {
  apps: [
    {
      name: "ai-runner",
      script: "go",
      args: "run .",
      cwd: "./runner",
      env: {
        PORT: 8081,
        JWT_SECRET: "super_secret_key_123",
        RUNNER_ADMIN_USER: "admin",
        RUNNER_ADMIN_PASS: "123456",
        R2_ACCOUNT_ID: "",
        R2_ACCESS_KEY_ID: "",
        R2_SECRET_ACCESS_KEY: "",
        R2_BUCKET_NAME: "",
        R2_PUBLIC_DOMAIN: ""
      }
    }
  ]
}

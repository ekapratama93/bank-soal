import { defineRailway, github, preserve, project, service } from "railway/iac";

export default defineRailway(() => {
  const backend = service("backend", {
    source: github("ekapratama93/bank-soal", {
      branch: "main",
      rootDirectory: "backend",
    }),
    healthcheck: "/api/health",
    healthcheckTimeout: 100,
    env: {
      PORT: "8000",
      OPENROUTER_API_KEY: preserve(),
      SUPABASE_URL: preserve(),
      SUPABASE_SERVICE_KEY: preserve(),
      DATABASE_URL: preserve(),
    },
  });

  const frontend = service("frontend", {
    source: github("ekapratama93/bank-soal", {
      branch: "main",
      rootDirectory: "frontend",
    }),
    healthcheck: "/",
    healthcheckTimeout: 50,
    env: {
      BACKEND_HOST: backend.env.RAILWAY_PRIVATE_DOMAIN,
      BACKEND_PORT: backend.env.PORT,
    },
  });

  return project("bank-soal", {
    resources: [backend, frontend],
  });
});
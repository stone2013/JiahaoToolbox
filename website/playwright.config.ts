import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
  testDir: './tests', testMatch: '**/*.spec.ts', fullyParallel: false, workers: 2,
  timeout: 30000, expect: { timeout: 7000 },
  reporter: [['list'], ['json', { outputFile: 'test-results/results.json' }]],
  use: { baseURL:'http://127.0.0.1:4174/JiahaoToolbox/', trace:'retain-on-failure', screenshot:'only-on-failure', launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE } },
  projects: [
    { name:'desktop', use:{ viewport:{width:1440,height:1000} } },
    { name:'tablet', use:{ viewport:{width:834,height:1112}, hasTouch:true } },
    { name:'mobile', use:{ ...devices['iPhone 13'], browserName:'chromium' } },
  ],
  webServer: { command:'npm run preview', url:'http://127.0.0.1:4174/JiahaoToolbox/', reuseExistingServer:!process.env.CI },
});

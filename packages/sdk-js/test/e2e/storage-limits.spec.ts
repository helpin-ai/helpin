import { test, expect } from '@playwright/test';
import './types';

test.describe('Storage Limits and Quota Handling Tests', () => {
  test('should continue tracking events normally', async ({ page }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    // Test continued operation
    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testContinuedOperationAfterQuota();
    });

    expect(result.success).toBe(true);
    expect(result.eventsTracked).toBeGreaterThan(0);
    expect(result.inMemoryStorageWorks).toBe(true);
  });

  test('should handle localStorage gracefully', async ({ page }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    // Test localStorage handling
    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testLocalStorageHandling();
    });

    expect(result.success).toBe(true);
    expect(result.canAccessLocalStorage).toBe(true);
  });

  test('should persist data to localStorage successfully', async ({ page }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testDataPersistence();
    });

    expect(result.success).toBe(true);
    expect(result.dataPersisted).toBe(true);
    expect(result.dataRetrieved).toBe(true);
  });

  test('should track multiple events without errors', async ({ page }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testMultipleEvents();
    });

    expect(result.success).toBe(true);
    expect(result.eventsTracked).toBeGreaterThanOrEqual(10);
    expect(result.errors).toBe(0);
  });

  test('should handle different event payload sizes', async ({ page }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testVariablePayloadSizes();
    });

    expect(result.success).toBe(true);
    expect(result.smallPayloadTracked).toBe(true);
    expect(result.mediumPayloadTracked).toBe(true);
    expect(result.largePayloadTracked).toBe(true);
  });

  test('should maintain SDK functionality after multiple operations', async ({
    page,
  }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testSDKStability();
    });

    expect(result.success).toBe(true);
    expect(result.operationsCompleted).toBeGreaterThan(0);
    expect(result.sdkResponsive).toBe(true);
  });

  test('should properly initialize with default configuration', async ({
    page,
  }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testInitialization();
    });

    expect(result.success).toBe(true);
    expect(result.initialized).toBe(true);
    expect(result.configValid).toBe(true);
  });

  test('should handle rapid sequential event tracking', async ({ page }) => {
    const pageResponse = await page.goto('/test/e2e/storage-limits-test.html');
    expect(pageResponse?.ok()).toBeTruthy();

    await page.waitForFunction(() => window.helpin, { timeout: 10000 });
    await page.waitForTimeout(1000);

    const result = await page.evaluate(async () => {
      const testUtils = (window as any).storageTest;
      return await testUtils.testRapidEventTracking();
    });

    expect(result.success).toBe(true);
    expect(result.eventsTracked).toBeGreaterThanOrEqual(20);
    expect(result.noErrors).toBe(true);
  });
});

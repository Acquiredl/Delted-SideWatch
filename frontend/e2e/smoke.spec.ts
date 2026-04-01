import { test, expect } from '@playwright/test'

// ---------------------------------------------------------------------------
// These smoke tests verify that pages render, navigation works, and key UI
// elements are present. They run against `next dev` without a backend — API
// calls fail gracefully via SWR loading/error states.
// ---------------------------------------------------------------------------

test.describe('Navigation', () => {
  test('all nav links are present and correct', async ({ page }) => {
    await page.goto('/')
    const nav = page.locator('nav')

    await expect(nav.getByRole('link', { name: 'Home' })).toHaveAttribute('href', '/')
    await expect(nav.getByRole('link', { name: 'Miner' })).toHaveAttribute('href', '/miner')
    await expect(nav.getByRole('link', { name: 'Blocks' })).toHaveAttribute('href', '/blocks')
    await expect(nav.getByRole('link', { name: 'Sidechain' })).toHaveAttribute('href', '/sidechain')
    await expect(nav.getByRole('link', { name: 'Fund' })).toHaveAttribute('href', '/fund')
    await expect(nav.getByRole('link', { name: 'Connect' })).toHaveAttribute('href', '/connect')
    await expect(nav.getByRole('link', { name: 'Subscribe' })).toHaveAttribute('href', '/subscribe')
  })

  test('clicking nav links navigates between pages', async ({ page }) => {
    await page.goto('/')

    await page.getByRole('link', { name: 'Blocks' }).click()
    await expect(page).toHaveURL('/blocks')
    await expect(page.getByRole('heading', { name: 'Blocks Found' })).toBeVisible()

    await page.getByRole('link', { name: 'Sidechain' }).click()
    await expect(page).toHaveURL('/sidechain')
    await expect(page.getByRole('heading', { name: 'Sidechain Shares' })).toBeVisible()

    await page.getByRole('link', { name: 'Home' }).click()
    await expect(page).toHaveURL('/')
  })
})

test.describe('Home page', () => {
  test('renders heading and description', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: /SideWatch/ })).toBeVisible()
    await expect(page.getByText('Decentralized Monero mining')).toBeVisible()
  })

  test('has correct page title', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveTitle(/SideWatch|P2Pool/)
  })

  test('footer is visible with privacy link', async ({ page }) => {
    await page.goto('/')
    const footer = page.locator('footer')
    await expect(footer).toBeVisible()
    await expect(footer.getByRole('link', { name: 'Privacy Notice' })).toHaveAttribute('href', '/privacy')
  })
})

test.describe('Miner page', () => {
  test('renders address lookup form', async ({ page }) => {
    await page.goto('/miner')
    await expect(page.getByRole('heading', { name: 'Miner Dashboard' })).toBeVisible()
    await expect(page.getByPlaceholder(/wallet address/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Look Up' })).toBeVisible()
  })

  test('shows prompt when no address entered', async ({ page }) => {
    await page.goto('/miner')
    await expect(page.getByText('Enter your wallet address above')).toBeVisible()
  })

  test('submitting an address triggers lookup', async ({ page }) => {
    await page.goto('/miner')
    const input = page.getByPlaceholder(/wallet address/i)
    await input.fill('4TestAddress123')
    await page.getByRole('button', { name: 'Look Up' }).click()

    // After submit, the prompt text should disappear (replaced by loading or error)
    await expect(page.getByText('Enter your wallet address above')).not.toBeVisible()
  })
})

test.describe('Blocks page', () => {
  test('renders heading and pagination controls', async ({ page }) => {
    await page.goto('/blocks')
    await expect(page.getByRole('heading', { name: 'Blocks Found' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Previous' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Next' })).toBeVisible()
  })

  test('previous button is disabled on first page', async ({ page }) => {
    await page.goto('/blocks')
    await expect(page.getByRole('button', { name: 'Previous' })).toBeDisabled()
  })
})

test.describe('Sidechain page', () => {
  test('renders heading and description', async ({ page }) => {
    await page.goto('/sidechain')
    await expect(page.getByRole('heading', { name: 'Sidechain Shares' })).toBeVisible()
    await expect(page.getByText('shares submitted to the P2Pool sidechain')).toBeVisible()
  })
})

test.describe('Subscribe page', () => {
  test('renders subscription info and address form', async ({ page }) => {
    await page.goto('/subscribe')
    await expect(page.getByRole('heading', { name: 'Support SideWatch' })).toBeVisible()
    await expect(page.getByText('$1+/mo Supporter')).toBeVisible()
    await expect(page.getByPlaceholder(/wallet address/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Look Up' })).toBeVisible()
  })

  test('shows prompt when no address entered', async ({ page }) => {
    await page.goto('/subscribe')
    await expect(page.getByText('Enter your wallet address above to view your subscription status')).toBeVisible()
  })

  test('submitting address shows subscription details', async ({ page }) => {
    await page.goto('/subscribe')
    await page.getByPlaceholder(/wallet address/i).fill('4TestAddress123')
    await page.getByRole('button', { name: 'Look Up' }).click()

    // Free tier limits should appear after address submission (default status is free)
    await expect(page.getByText('Free tier limits:')).toBeVisible()
  })
})

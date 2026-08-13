#!/usr/bin/env node

const secretKey = process.env.STRIPE_SECRET_KEY?.trim();

if (!secretKey) {
  console.error('STRIPE_SECRET_KEY is required.');
  process.exit(1);
}

const stripeVersion = process.env.STRIPE_API_VERSION || '2026-02-25.clover';
const baseURL = 'https://api.stripe.com/v1';

async function stripeRequest(path, params) {
  const body = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null) body.append(key, String(value));
  }

  const response = await fetch(`${baseURL}${path}`, {
    method: 'POST',
    headers: {
      Authorization: `Basic ${Buffer.from(`${secretKey}:`).toString('base64')}`,
      'Content-Type': 'application/x-www-form-urlencoded',
      'Stripe-Version': stripeVersion,
    },
    body,
  });

  const payload = await response.json();
  if (!response.ok) {
    const message = payload?.error?.message || response.statusText;
    throw new Error(`${path}: ${message}`);
  }
  return payload;
}

async function createProduct(name, description) {
  return stripeRequest('/products', {
    name,
    description,
  });
}

async function createRecurringPrice(productId, nickname, amountCents, interval) {
  return stripeRequest('/prices', {
    product: productId,
    nickname,
    currency: 'usd',
    unit_amount: amountCents,
    'recurring[interval]': interval,
  });
}

const products = {
  starter: await createProduct('Helpin Starter', 'Helpin Starter workspace subscription'),
  growth: await createProduct('Helpin Growth', 'Helpin Growth workspace subscription'),
};

const prices = {
  STRIPE_STARTER_MONTHLY_PRICE_ID: await createRecurringPrice(products.starter.id, 'Starter monthly', 9900, 'month'),
  STRIPE_STARTER_ANNUAL_PRICE_ID: await createRecurringPrice(products.starter.id, 'Starter annual', 94800, 'year'),
  STRIPE_GROWTH_MONTHLY_PRICE_ID: await createRecurringPrice(products.growth.id, 'Growth monthly', 29900, 'month'),
  STRIPE_GROWTH_ANNUAL_PRICE_ID: await createRecurringPrice(products.growth.id, 'Growth annual', 286800, 'year'),
};

console.log('Stripe billing Products and Prices created.');
console.log('');
for (const [envName, price] of Object.entries(prices)) {
  console.log(`${envName}=${price.id}`);
}

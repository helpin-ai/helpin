export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-3xl px-6 lg:px-8 py-20">
      <h1 className="text-3xl font-bold tracking-tight text-foreground mb-4">Privacy Policy</h1>
      <p className="text-sm text-muted-foreground mb-12">Last updated: June 23, 2026</p>

      <div className="prose prose-sm max-w-none text-muted-foreground space-y-8">
        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">Who We Are</h2>
          <p className="leading-relaxed">
            Helpin is operated by Usermaven Inc. When this policy says "Helpin," "we," "us," or "our," it refers to Usermaven Inc. and the Helpin service.
          </p>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">1. Information We Collect</h2>
          <p className="leading-relaxed mb-3">When you use Helpin, we collect information you provide directly:</p>
          <ul className="list-disc pl-5 space-y-1">
            <li>Account information (name, email address, company name)</li>
            <li>Workspace data (projects, tasks, documents, conversations)</li>
            <li>Usage data (feature interactions, agent runs, API calls)</li>
            <li>Communication data (support requests, feedback)</li>
            <li>Billing information needed to manage your subscription, invoices, and payment status</li>
          </ul>
          <p className="leading-relaxed mt-3">We also collect technical data automatically: IP address, browser type, device information, and cookies for authentication and analytics.</p>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">2. How We Use Your Information</h2>
          <p className="leading-relaxed mb-3">We use your information to:</p>
          <ul className="list-disc pl-5 space-y-1">
            <li>Provide, maintain, and improve Helpin services</li>
            <li>Power AI agents with your workspace context</li>
            <li>Send service-related communications</li>
            <li>Monitor and prevent security issues</li>
            <li>Process subscriptions, invoices, upgrades, downgrades, and payment-related notices</li>
            <li>Comply with legal obligations</li>
          </ul>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">3. AI and Your Data</h2>
          <p className="leading-relaxed mb-3">Helpin AI agents operate within your workspace using your data to perform tasks. Important commitments:</p>
          <ul className="list-disc pl-5 space-y-1">
            <li>Your data is never used to train AI models</li>
            <li>Your data is never shared across workspaces</li>
            <li>You bring your own AI provider API keys — we do not store or access your model outputs beyond what is needed to complete agent tasks</li>
            <li>Agent actions can be configured to require human approval before execution</li>
          </ul>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">4. Data Security</h2>
          <p className="leading-relaxed">Your data is encrypted at rest and in transit using industry-standard encryption (AES-256, TLS 1.3). We implement access controls, audit logging, and regular security reviews to protect your information.</p>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">5. Data Sharing</h2>
          <p className="leading-relaxed mb-3">We do not sell your data. We share information only with:</p>
          <ul className="list-disc pl-5 space-y-1">
            <li>Infrastructure providers (hosting, storage) under strict data processing agreements</li>
            <li>Payment processors, including Stripe, to process payments, manage subscriptions, prevent fraud, and provide invoices</li>
            <li>Analytics tools (Usermaven) for product improvement — anonymized where possible</li>
            <li>AI model providers selected or configured for your workspace, only as needed to complete agent tasks</li>
            <li>Law enforcement when legally required</li>
          </ul>
          <p className="leading-relaxed mt-3">We do not store full payment card numbers on our servers. Payment details are handled by our payment processor.</p>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">6. Data Retention</h2>
          <p className="leading-relaxed">We retain your data for as long as your account is active. When you delete your account, we remove your workspace data within 30 days. Some data may be retained longer for legal compliance or fraud prevention.</p>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">7. Your Rights</h2>
          <p className="leading-relaxed mb-3">You have the right to:</p>
          <ul className="list-disc pl-5 space-y-1">
            <li>Access and export your data</li>
            <li>Correct inaccurate information</li>
            <li>Delete your account and data</li>
            <li>Object to data processing</li>
            <li>Withdraw consent at any time</li>
          </ul>
        </section>

        <section>
          <h2 className="text-lg font-semibold text-foreground mb-3">8. Contact</h2>
          <p className="leading-relaxed mb-3">For privacy-related questions, contact us at <a href="mailto:privacy@helpin.ai" className="text-foreground underline">privacy@helpin.ai</a>.</p>
          <address className="not-italic leading-relaxed">
            Usermaven Inc.<br />
            16192 Coastal Highway<br />
            Lewes, DE 19958<br />
            United States
          </address>
        </section>
      </div>
    </div>
  );
}

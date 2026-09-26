export default function TermsPage() {
  return (
    <article className="legal-page">
      <div className="legal-page-inner">
        <h1 className="legal-title">Terms of Service</h1>
        <p className="legal-updated">Last updated: June 23, 2026</p>

        <div className="legal-content">
          <section>
            <h2 className="legal-section-title">1. Acceptance of Terms</h2>
            <p className="leading-relaxed">By accessing or using Helpin, you agree to be bound by these Terms of Service. Helpin is operated by Usermaven Inc. If you are using Helpin on behalf of an organization, you represent that you have the authority to bind that organization to these terms.</p>
          </section>

          <section>
            <h2 className="legal-section-title">2. Description of Service</h2>
            <p className="leading-relaxed">Helpin is an AI-powered operating system that brings project management, documentation, customer support, CRM, and knowledge management into one connected platform. AI agents operate within your workspace to automate and assist with work across these functions.</p>
          </section>

          <section>
            <h2 className="legal-section-title">3. Accounts</h2>
            <p className="leading-relaxed mb-3">You are responsible for:</p>
            <ul className="list-disc pl-5 space-y-1">
              <li>Maintaining the security of your account credentials</li>
              <li>All activity that occurs under your account</li>
              <li>Ensuring your team members comply with these terms</li>
              <li>Providing accurate and up-to-date information</li>
            </ul>
          </section>

          <section>
            <h2 className="legal-section-title">4. Your Data</h2>
            <p className="leading-relaxed mb-3">You retain ownership of all data you upload to Helpin. By using the service, you grant us a limited license to process your data as needed to provide the service, including:</p>
            <ul className="list-disc pl-5 space-y-1">
              <li>Storing and displaying your workspace content</li>
              <li>Processing data through AI agents you configure</li>
              <li>Creating backups for data protection</li>
            </ul>
            <p className="leading-relaxed mt-3">We do not use your data to train AI models. Your data is never shared across workspaces.</p>
          </section>

          <section>
            <h2 className="legal-section-title">5. AI Agents</h2>
            <p className="leading-relaxed mb-3">Helpin AI agents perform actions within your workspace based on your configuration. You acknowledge that:</p>
            <ul className="list-disc pl-5 space-y-1">
              <li>AI agents may produce imperfect results and should be reviewed</li>
              <li>You are responsible for configuring appropriate approval modes</li>
              <li>Agent actions taken with your approval are your responsibility</li>
              <li>You provide your own API keys for AI model providers</li>
            </ul>
          </section>

          <section>
            <h2 className="legal-section-title">6. Acceptable Use</h2>
            <p className="leading-relaxed mb-3">You agree not to:</p>
            <ul className="list-disc pl-5 space-y-1">
              <li>Use Helpin for any illegal purpose</li>
              <li>Attempt to gain unauthorized access to our systems</li>
              <li>Interfere with the service or other users</li>
              <li>Reverse engineer or copy any part of the service</li>
              <li>Use the service to build a competing product</li>
            </ul>
          </section>

          <section>
            <h2 className="legal-section-title">7. Payments and Subscriptions</h2>
            <p className="leading-relaxed mb-3">
              Paid plans are billed according to the plan, billing period, and usage terms shown at checkout or in the billing area. By purchasing a paid plan, you authorize us and our payment processor, Stripe, to charge the payment method you provide for recurring subscription fees, applicable taxes, and any usage-based charges.
            </p>
            <ul className="list-disc pl-5 space-y-1">
              <li>You are responsible for keeping billing and payment information accurate and up to date</li>
              <li>Subscriptions renew automatically unless canceled before the next renewal date</li>
              <li>Failed or unpaid invoices may result in reduced access, workspace locking, or suspension until payment is resolved</li>
              <li>Fees are non-refundable except where required by law or explicitly stated by us</li>
            </ul>
          </section>

          <section>
            <h2 className="legal-section-title">8. Service Availability</h2>
            <p className="leading-relaxed">We strive for high availability but do not guarantee uninterrupted service. We may perform maintenance, updates, or experience outages. We will notify you of planned downtime when possible.</p>
          </section>

          <section>
            <h2 className="legal-section-title">9. Limitation of Liability</h2>
            <p className="leading-relaxed">To the maximum extent permitted by law, Helpin shall not be liable for any indirect, incidental, special, consequential, or punitive damages arising from your use of the service. Our total liability is limited to the amount you paid us in the 12 months preceding the claim.</p>
          </section>

          <section>
            <h2 className="legal-section-title">10. Governing Law</h2>
            <p className="leading-relaxed">These terms are governed by the laws of the State of Delaware, United States, without regard to conflict of law principles.</p>
          </section>

          <section>
            <h2 className="legal-section-title">11. Changes to Terms</h2>
            <p className="leading-relaxed">We may update these terms from time to time. We will notify you of material changes via email or in-app notification. Continued use of Helpin after changes constitutes acceptance of the updated terms.</p>
          </section>

          <section>
            <h2 className="legal-section-title">12. Contact</h2>
            <p className="leading-relaxed mb-3">For questions about these terms, contact us at <a href="mailto:legal@helpin.ai" className="legal-link">legal@helpin.ai</a>.</p>
            <address className="not-italic leading-relaxed">
              Usermaven Inc.<br />
              16192 Coastal Highway<br />
              Lewes, DE 19958<br />
              United States
            </address>
          </section>
        </div>
      </div>
    </article>
  );
}

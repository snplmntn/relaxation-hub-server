-- Migration 065: Seed policy keys used by /api/v1/content/{key}

INSERT INTO legal_documents (doc_key, title, content_markdown, version, effective_at, updated_at)
VALUES
(
    'terms_and_conditions',
    'Terms and Conditions',
    $$<h1>Terms and Conditions</h1>
<p>By booking services with Relaxation Hub, you agree to these terms and conditions.</p>
<h2>1. Service Scope</h2>
<p>Services are fulfilled based on the booking details you provide. Please review your selections before confirming.</p>
<h2>2. Client Responsibilities</h2>
<p>Provide accurate location, contact, and special instructions to help ensure successful service delivery.</p>
<h2>3. Cancellations and No-Shows</h2>
<p>Late cancellations and no-shows may be subject to policy enforcement under platform rules.</p>
<h2>4. Liability</h2>
<p>Relaxation Hub is not liable for delays caused by force majeure events, traffic disruptions, or other events beyond reasonable control.</p>
<p>For support, contact us through the in-app support channels.</p>$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'privacy_policy',
    'Privacy Policy',
    $$<h1>Privacy Policy</h1>
<p>Relaxation Hub values your privacy. We only collect information needed to operate and improve our services.</p>
<h2>1. Data We Collect</h2>
<p>We may collect account, booking, location, and device-related information required to provide service functionality.</p>
<h2>2. Data Usage</h2>
<p>Data is used for booking fulfillment, safety operations, communications, support, and service improvements.</p>
<h2>3. Data Protection</h2>
<p>We use reasonable security controls to protect your personal data and restrict unauthorized access.</p>
<p>For support, contact us through the in-app support channels.</p>$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'refund_policy',
    'Refund Policy',
    $$<h1>Refund Policy</h1>
<p>Relaxation Hub reviews refund requests fairly based on booking records and reported incidents.</p>
<h2>1. Request Window</h2>
<p>Please submit refund concerns as soon as possible after service completion or issue occurrence.</p>
<h2>2. Eligibility</h2>
<p>Approved refunds depend on verification, booking details, and policy compliance.</p>
<h2>3. Resolution</h2>
<p>Depending on the case, resolution may include partial refund, full refund, credit, or other remediation.</p>
<p>For support, contact us through the in-app support channels.</p>$$,
    '1.0.0',
    NOW(),
    NOW()
)
ON CONFLICT (doc_key) DO NOTHING;

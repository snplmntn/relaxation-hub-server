-- Add Daily Sales remittance and Therapist Salary adjustment report tables.

CREATE TABLE IF NOT EXISTS daily_sales_remittances (
    remittance_id SERIAL PRIMARY KEY,
    business_date DATE NOT NULL,
    branch_id INT NOT NULL REFERENCES branches(branch_id) ON DELETE RESTRICT,
    bill_1000 INT NOT NULL DEFAULT 0 CHECK (bill_1000 >= 0),
    bill_500 INT NOT NULL DEFAULT 0 CHECK (bill_500 >= 0),
    bill_200 INT NOT NULL DEFAULT 0 CHECK (bill_200 >= 0),
    bill_100 INT NOT NULL DEFAULT 0 CHECK (bill_100 >= 0),
    bill_50 INT NOT NULL DEFAULT 0 CHECK (bill_50 >= 0),
    bill_20 INT NOT NULL DEFAULT 0 CHECK (bill_20 >= 0),
    bill_10 INT NOT NULL DEFAULT 0 CHECK (bill_10 >= 0),
    bill_5 INT NOT NULL DEFAULT 0 CHECK (bill_5 >= 0),
    bill_1 INT NOT NULL DEFAULT 0 CHECK (bill_1 >= 0),
    actual_remitted NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (actual_remitted >= 0),
    tips_total NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (tips_total >= 0),
    client_funds_used NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (client_funds_used >= 0),
    client_funds_added NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (client_funds_added >= 0),
    remitted_to_mark NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (remitted_to_mark >= 0),
    other_remitted_amount NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (other_remitted_amount >= 0),
    remitted_to TEXT,
    others_deducted NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (others_deducted >= 0),
    others_added NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (others_added >= 0),
    notes TEXT,
    created_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (business_date, branch_id)
);

CREATE INDEX IF NOT EXISTS idx_daily_sales_remittances_business_date ON daily_sales_remittances(business_date);
CREATE INDEX IF NOT EXISTS idx_daily_sales_remittances_branch_date ON daily_sales_remittances(branch_id, business_date);

CREATE TABLE IF NOT EXISTS therapist_payroll_adjustments (
    adjustment_id SERIAL PRIMARY KEY,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    adjustment_date DATE NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('add', 'minus')),
    category VARCHAR(30) NOT NULL CHECK (category IN ('benefits', 'cash_advance', 'salary', 'correction', 'parcel', 'absence', 'other')),
    amount NUMERIC(10,2) NOT NULL CHECK (amount >= 0),
    reason TEXT NOT NULL,
    cash_movement NUMERIC(10,2) NOT NULL DEFAULT 0,
    created_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (period_end >= period_start)
);

CREATE INDEX IF NOT EXISTS idx_therapist_payroll_adjustments_period ON therapist_payroll_adjustments(period_start, period_end) WHERE voided_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_therapist_payroll_adjustments_therapist ON therapist_payroll_adjustments(therapist_id) WHERE voided_at IS NULL;

CREATE TABLE payments (
    id SERIAL PRIMARY KEY,

    barbershop_id BIGINT NOT NULL,
    appointment_id BIGINT NOT NULL UNIQUE,

    amount NUMERIC(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL,

    paid_at TIMESTAMP,
    expired_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_payments_barbershop
        FOREIGN KEY (barbershop_id)
        REFERENCES barbershops(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_payments_appointment
        FOREIGN KEY (appointment_id)
        REFERENCES appointments(id)
        ON DELETE CASCADE
);

CREATE TABLE pix_events (
    id SERIAL PRIMARY KEY,
    txid VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT ux_pix_event UNIQUE (txid, event_type)
);


CREATE INDEX idx_payments_status ON payments(status);

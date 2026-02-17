CREATE TABLE IF NOT EXISTS favorite_therapists (
    user_id BIGINT NOT NULL,
    therapist_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, therapist_id),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (therapist_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_favorite_therapists_user_id ON favorite_therapists(user_id);

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS social_profiles (
    user_id UUID PRIMARY KEY,
    total_friends INTEGER DEFAULT 0 NOT NULL CHECK (total_friends >= 0),
    total_ratings INTEGER DEFAULT 0 NOT NULL CHECK (total_ratings >= 0),
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE TABLE IF NOT EXISTS friends (
    user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
    friend_id UUID NOT NULL, 
    added_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,

    PRIMARY KEY (user_id, friend_id)
);

CREATE TABLE IF NOT EXISTS film_ratings (
    grade_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
    film_id INTEGER NOT NULL,
    grade INTEGER NOT NULL CHECK (grade >= 0 AND grade <= 5),
    review TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,

    CONSTRAINT unique_user_film_rating UNIQUE (user_id, film_id)
);

CREATE INDEX IF NOT EXISTS idx_ratings_film_id ON film_ratings(film_id);

CREATE INDEX IF NOT EXISTS idx_ratings_user_id ON film_ratings(user_id);

CREATE INDEX IF NOT EXISTS idx_friends_friend_id ON friends(friend_id);

CREATE TABLE IF NOT EXISTS friend_requests (
    request_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
    to_user_id UUID NOT NULL REFERENCES social_profiles(user_id) ON DELETE CASCADE,
    from_username VARCHAR(255) NOT NULL, 
    status VARCHAR(20) NOT NULL DEFAULT 'pending', 
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,

    CONSTRAINT unique_pending_request UNIQUE (from_user_id, to_user_id)
);
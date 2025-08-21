CREATE TABLE IF NOT EXISTS tasks (
    id text NOT NULL, 
    value bytea,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT tasks_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS tasks (
    id text NOT NULL, 
    value bytea, 
    CONSTRAINT tasks_pkey PRIMARY KEY (id)
);

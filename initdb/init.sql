-- TODO: Move to .go ?
-- TODO: When is NOT NULL required?


CREATE TABLE users (
       --id SERIAL PRIMARY KEY,
       name TEXT PRIMARY KEY NOT NULL
);

-- hard-code users
INSERT INTO users (name) VALUES('miles');
INSERT INTO users (name) VALUES('bill');
INSERT INTO users (name) VALUES('john');

CREATE TABLE authors (
       id SERIAL PRIMARY KEY,
       name TEXT NOT NULL,
       date_of_birth date NOT NULL
);

CREATE TABLE books (
       id SERIAL PRIMARY KEY,
       author_id INT NOT NULL,
       title TEXT NOT NULL,
       published timestamp NOT NULL,
       CONSTRAINT fk_author FOREIGN KEY (author_id) REFERENCES authors (id)
);


CREATE TABLE events (
       ts TIMESTAMP NOT NULL,   -- when
       username TEXT NOT NULL,  -- who
       operation TEXT NOT NULL, -- CREATE, READ, UPDATE or DELETE
       obj_type TEXT NOT NULL,  --
       obj_id INT,              -- if applicable
       data TEXT                -- TODO: Data (json?) if op is CREATE or UPDATE
       -- CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users (id)
);

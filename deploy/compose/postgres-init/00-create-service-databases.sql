-- Development-only bootstrap. Each service owns a database and role even when
-- all databases share one local PostgreSQL instance.
CREATE ROLE conduit_auth LOGIN PASSWORD 'auth';
CREATE DATABASE conduit_auth OWNER conduit_auth;

CREATE ROLE conduit_profile LOGIN PASSWORD 'profile';
CREATE DATABASE conduit_profile OWNER conduit_profile;

CREATE ROLE conduit_posts LOGIN PASSWORD 'posts';
CREATE DATABASE conduit_posts OWNER conduit_posts;

CREATE ROLE conduit_comments LOGIN PASSWORD 'comments';
CREATE DATABASE conduit_comments OWNER conduit_comments;

CREATE ROLE conduit_subscriptions LOGIN PASSWORD 'subscriptions';
CREATE DATABASE conduit_subscriptions OWNER conduit_subscriptions;

DROP TRIGGER IF EXISTS users_create_support_thread ON users;
DROP FUNCTION IF EXISTS create_support_thread_for_user();
DROP TABLE IF EXISTS support_read_states;
DROP TABLE IF EXISTS support_messages;
DROP TABLE IF EXISTS support_thread_moderators;
DROP TABLE IF EXISTS support_threads;
DROP TYPE IF EXISTS support_message_sender_type;

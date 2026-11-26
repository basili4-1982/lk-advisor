-- Demo-only trace of agent answers: question, report, base/LLM answer, guard.
CREATE TABLE IF NOT EXISTS agent_calls (
    id           bigserial PRIMARY KEY,
    at           timestamptz NOT NULL DEFAULT now(),
    session_id   text NOT NULL,
    client_id    text NOT NULL,
    question     text NOT NULL,
    intent       text NOT NULL,
    tool_calls   jsonb NOT NULL DEFAULT '[]'::jsonb,
    evidence     jsonb NOT NULL DEFAULT '[]'::jsonb,
    report       jsonb NOT NULL DEFAULT '{}'::jsonb,
    base_answer  text NOT NULL DEFAULT '',
    llm_answer   text NOT NULL DEFAULT '',
    final_answer text NOT NULL DEFAULT '',
    guard_error  text NOT NULL DEFAULT '',
    fallback     boolean NOT NULL DEFAULT false,
    latency_ms   integer NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS agent_calls_client_idx ON agent_calls (client_id, at DESC);

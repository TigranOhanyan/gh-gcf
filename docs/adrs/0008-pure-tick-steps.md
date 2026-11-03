# Tick steps are pure

A tick step given the same state does the same work. A launch claim stores a Cursor agent id of the form `bc-<uuid>`. Creating the agent sends that id. Repeating the create returns `409 agent_id_conflict` instead of a second agent, and the tick records the ack from `GET /v1/agents/{id}`. A claim with no ack is still waiting, so the next tick repeats the same create.

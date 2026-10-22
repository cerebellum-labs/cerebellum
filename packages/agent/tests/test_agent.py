from __future__ import annotations

import time

import pytest
from conftest import RULES, jpeg, wait_for

from cerebellum import Correction, Decision, RulesError
from cerebellum.clock import uuid7


def test_reversible_decision_is_made_on_the_device_and_synced(server, make_agent):
    agent = make_agent()
    agent.start()
    d = agent.decide(
        "sort_item", proposed_action="basket", frame=jpeg(), local_model="yolox-nano",
        local_confidence=0.93, local_latency_ms=30, metadata={"bbox": [1, 2, 3, 4]},
    )  # fmt: skip
    assert d.status == "decided_on_device"
    assert (d.action, d.source, d.model, d.confidence) == ("basket", "device", "yolox-nano", 0.93)
    assert not d.needs_review

    done = agent.mark_executed(d.id)
    assert done.status == "executed" and done.revision == 2
    wait_for(lambda: server.decisions.get(d.id, {}).get("executed_at") is not None)
    wait_for(lambda: agent.get(d.id).revision == 2 and agent.get(d.id).received_at is not None)

    sent = server.decisions[d.id]
    assert sent["prompt"] == RULES["tasks"]["sort_item"]["prompt"]  # kept for a later review
    assert sent["metadata"] == {"bbox": [1, 2, 3, 4]}
    assert server.frame_uploads[d.id] == 1  # the frame goes once, however many revisions
    assert server.agent_version == "0.1.0"
    assert agent.is_online


def test_reversible_without_confidence_is_reviewed(make_agent):
    agent = make_agent()
    assert agent.decide("sort_item", proposed_action="basket").needs_review
    assert agent.decide("sort_item", proposed_action="basket", local_confidence=0.5).needs_review
    assert not agent.decide("sort_item", proposed_action="basket", local_confidence=0.8).needs_review


def test_irreversible_waits_for_the_server_without_blocking(server, make_agent):
    server.decide_action = "discard"
    answers: list[Decision] = []
    agent = make_agent()
    agent.on_decision(answers.append)
    d = agent.decide("sort_item", proposed_action="discard", frame=jpeg(), local_confidence=0.9)
    assert d.status == "waiting_for_server" and d.action is None
    assert d.action_class == "irreversible"

    agent.start()
    wait_for(lambda: answers)
    a = answers[0]
    assert a.id == d.id and a.status == "decided_on_server"
    assert (a.action, a.source, a.model, a.reason) == ("discard", "server", "fake-model", "because")
    assert a.cost_usd == 0.0004

    assert agent.mark_executed(d.id).status == "executed"
    wait_for(lambda: server.decisions[d.id].get("executed_at"))
    assert server.frame_uploads[d.id] == 1


def test_no_proposal_goes_to_the_server_as_irreversible(server, make_agent):
    answers: list[Decision] = []
    agent = make_agent()
    agent.on_decision(answers.append)
    agent.start()
    d = agent.decide("sort_item")
    assert d.action_class == "irreversible" and d.proposed_action is None
    wait_for(lambda: answers)
    assert answers[0].action == "basket"


def test_offline_waits_instead_of_giving_up(server, make_agent):
    server.online = False
    answers: list[Decision] = []
    connection: list[bool] = []
    agent = make_agent(max_attempts=2)
    agent.on_decision(answers.append)
    agent.on_connection(connection.append)
    agent.start()
    d = agent.decide("sort_item", proposed_action="discard", frame=jpeg())

    wait_for(lambda: server.count("/api/v1/decide") >= 5)  # many tries, more than max_attempts
    assert not answers and not agent.is_online
    assert agent.get(d.id).status == "waiting_for_server"
    assert agent.get(d.id).attempts == 0  # the network isn't the decision's fault

    server.online = True
    wait_for(lambda: answers)
    assert answers[0].status == "decided_on_server"
    assert agent.is_online and connection[-1] is True


def test_reversible_keeps_working_offline_and_syncs_later(server, make_agent):
    server.online = False
    agent = make_agent()
    agent.start()
    ids = []
    for _ in range(3):
        d = agent.decide("sort_item", proposed_action="basket", local_confidence=0.95, frame=jpeg())
        agent.mark_executed(d.id)
        ids.append(d.id)
    wait_for(lambda: server.count("/api/v1/sync") >= 3)
    assert not server.decisions

    server.online = True
    wait_for(lambda: all(i in server.decisions and server.decisions[i].get("executed_at") for i in ids))
    assert all(server.frame_uploads[i] == 1 for i in ids)


def test_server_failures_cancel_after_max_attempts(server, make_agent):
    server.fail_decide = 99
    answers: list[Decision] = []
    agent = make_agent(max_attempts=3)
    agent.on_decision(answers.append)
    agent.start()
    d = agent.decide("sort_item", proposed_action="discard")
    wait_for(lambda: answers)
    a = answers[0]
    assert a.id == d.id and a.status == "cancelled" and a.attempts == 3
    assert "3 tries" in a.cancel_reason
    wait_for(lambda: server.decisions[d.id].get("cancelled_at"))


def test_a_decision_the_server_turns_down_is_cancelled_at_once(server, make_agent):
    server.reject_decide = True
    answers: list[Decision] = []
    agent = make_agent()
    agent.on_decision(answers.append)
    agent.start()
    agent.decide("sort_item", proposed_action="discard")
    wait_for(lambda: answers)
    assert answers[0].status == "cancelled" and answers[0].attempts == 0


def test_cancel_while_waiting_never_calls_back(server, make_agent):
    server.online = False
    answers: list[Decision] = []
    agent = make_agent()
    agent.on_decision(answers.append)
    agent.start()
    d = agent.decide("sort_item", proposed_action="discard")
    c = agent.cancel(d.id, "The item left the zone.")
    assert c.status == "cancelled" and c.cancel_reason == "The item left the zone."
    with pytest.raises(RulesError):
        agent.cancel(d.id)

    server.online = True
    wait_for(lambda: server.decisions.get(d.id, {}).get("cancelled_at"))
    assert not answers
    assert agent.get(d.id).status == "cancelled"


def test_late_answer_keeps_the_decision_cancelled(server, make_agent):
    agent = make_agent()
    d = agent.decide("sort_item", proposed_action="discard")
    agent.cancel(d.id)
    # The server's answer arrives for a decision cancelled meanwhile.
    agent._store.apply_server(d.id, {"action": "discard", "source": "server", "model": "m"}, 1)
    assert agent.get(d.id).status == "cancelled"


def test_correction_reaches_the_robot_once_and_is_marked_applied(server, make_agent):
    server.review_action = "discard"
    corrections: list[Correction] = []
    agent = make_agent()
    agent.on_correction(corrections.append)
    agent.start()
    d = agent.decide("sort_item", proposed_action="basket", local_confidence=0.6, frame=jpeg())
    agent.mark_executed(d.id)

    wait_for(lambda: corrections)
    c = corrections[0]
    assert (c.decision_id, c.action, c.reason) == (d.id, "discard", "it is a ball")
    assert agent.get(d.id).status == "corrected"

    agent.mark_correction_applied(c.id)
    wait_for(lambda: server.decisions[d.id].get("correction_applied_at"))
    wait_for(lambda: server.count("/api/v1/sync") >= 4)
    assert len(corrections) == 1  # later syncs don't repeat it
    assert agent.get(d.id).correction_applied_at is not None


def test_reflex_is_journalled_as_executed(server, make_agent):
    agent = make_agent()
    agent.start()
    d = agent.report_reflex("safety", "stop", frame=jpeg(), local_model="yolox-nano", local_confidence=0.9)
    assert d.status == "executed" and d.action_class == "reflex" and d.options == ["stop"]
    wait_for(lambda: d.id in server.decisions)
    assert server.decisions[d.id]["executed_at"] is not None
    with pytest.raises(RulesError):
        agent.report_reflex("safety", "basket")
    with pytest.raises(RulesError):
        agent.decide("sort_item", proposed_action="stop")


def test_a_wrong_token_stops_the_agent_sending(server, make_agent):
    agent = make_agent(token="not-the-token")
    agent.start()
    agent.decide("sort_item", proposed_action="discard")
    wait_for(lambda: agent.auth_failed)
    n = len(server.requests)
    import time

    time.sleep(0.3)
    assert len(server.requests) == n  # no more requests with that token


def test_the_queue_survives_a_restart(server, make_agent):
    server.online = False
    first = make_agent()
    d = first.decide("sort_item", proposed_action="discard", frame=jpeg())
    first.stop()

    server.online = True
    answers: list[Decision] = []
    second = make_agent()
    second.on_decision(answers.append)
    assert second.get(d.id).status == "waiting_for_server"
    second.start()
    wait_for(lambda: answers)
    assert answers[0].id == d.id and server.frame_uploads[d.id] == 1


def test_calls_are_checked_against_the_rules(make_agent):
    agent = make_agent()
    with pytest.raises(RulesError):
        agent.decide("no_such_task")
    with pytest.raises(RulesError):
        agent.decide("sort_item", proposed_action="fly")
    with pytest.raises(RulesError):
        agent.decide("sort_item", local_confidence=1.5)
    with pytest.raises(RulesError):
        agent.decide("sort_item", metadata={"x": object()})
    with pytest.raises(RulesError):
        agent.mark_executed(uuid7())
    d = agent.decide("sort_item", proposed_action="discard")
    with pytest.raises(RulesError):
        agent.mark_executed(d.id)  # still waiting


def test_frames_the_server_has_are_deleted_and_unsent_ones_kept(server, make_agent):
    agent = make_agent(keep_frames_days=0)
    agent.start()
    sent = agent.decide("sort_item", proposed_action="basket", local_confidence=0.9, frame=jpeg())
    wait_for(lambda: sent.id in server.decisions)
    wait_for(lambda: agent._store.frame(sent.id) is None)

    server.online = False
    unsent = agent.decide("sort_item", proposed_action="basket", local_confidence=0.9, frame=jpeg())
    wait_for(lambda: server.count("/api/v1/sync") >= 3)
    assert agent._store.frame(unsent.id) is not None  # not sent yet, so not deleted


def test_an_answer_is_delivered_after_a_restart_if_the_robot_missed_it(server, make_agent):
    first = make_agent()  # no on_decision yet: the answer arrives with nobody to take it
    d = first.decide("sort_item", proposed_action="discard")
    first.start()
    wait_for(lambda: first.get(d.id).action is not None)
    first.stop()

    answers: list[Decision] = []
    second = make_agent()
    second.on_decision(answers.append)
    second.start()
    wait_for(lambda: answers)
    assert answers[0].id == d.id and answers[0].status == "decided_on_server"
    time.sleep(0.3)
    assert len(answers) == 1  # once delivered, not again


def test_an_answer_the_robot_already_executed_is_not_delivered_again(server, make_agent):
    first = make_agent()
    d = first.decide("sort_item", proposed_action="discard")
    first.start()
    wait_for(lambda: first.get(d.id).action is not None)
    first.mark_executed(d.id)
    first.stop()

    answers: list[Decision] = []
    second = make_agent()
    second.on_decision(answers.append)
    second.start()
    time.sleep(0.3)
    assert not answers


def test_an_open_correction_is_announced_again_after_a_restart(server, make_agent):
    server.review_action = "discard"
    first_seen: list[Correction] = []
    first = make_agent()
    first.on_correction(first_seen.append)
    first.start()
    d = first.decide("sort_item", proposed_action="basket", local_confidence=0.5)
    wait_for(lambda: first_seen)
    first.stop()

    again: list[Correction] = []
    second = make_agent()
    second.on_correction(again.append)
    second.start()
    wait_for(lambda: again)  # the robot has forgotten it, and the button wasn't pressed yet
    assert again[0].decision_id == d.id

    second.mark_correction_applied(again[0].id)
    second.stop()
    third_seen: list[Correction] = []
    third = make_agent()
    third.on_correction(third_seen.append)
    third.start()
    time.sleep(0.3)
    assert not third_seen  # applied: never again


def test_a_413_makes_the_batches_smaller(server, make_agent):
    server.online = False
    agent = make_agent(sync_batch=8)
    ids = [agent.decide("sort_item", proposed_action="basket", local_confidence=0.9).id for _ in range(6)]
    agent.start()
    server.max_items = 2
    server.online = True
    wait_for(lambda: all(i in server.decisions for i in ids))
    assert agent._effective_sync_batch <= 2


def test_a_request_the_server_rejects_is_not_sent_again(server, make_agent):
    server.reject_sync = True
    agent = make_agent()
    agent.start()
    agent.decide("sort_item", proposed_action="basket", local_confidence=0.9)
    wait_for(lambda: server.count("/api/v1/sync") >= 2)
    with_items = sum(1 for p, b in server.requests if p == "/api/v1/sync" and b["items"])
    time.sleep(0.4)
    assert sum(1 for p, b in server.requests if p == "/api/v1/sync" and b["items"]) == with_items == 1


def test_it_works_as_a_context_manager(server, make_agent):
    with make_agent() as agent:
        d = agent.decide("sort_item", proposed_action="basket", local_confidence=0.9)
        wait_for(lambda: d.id in server.decisions)

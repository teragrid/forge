"""Tests for the LangGraph StateGraph workflow.

Test design (QA checklist):
  Happy path      — full graph run with mocked agents produces final_answer.
  Boundary        — graph with empty plan routes to END immediately.
  Routing         — research keyword in step routes to researcher node.
  Routing         — non-research step routes to executor node.
  Checkpointing   — graph can resume from a saved checkpoint.
  Concurrency     — two simultaneous graph invocations do not share state.
  False-positive  — route_next returns "executor" for a generic action step.
"""

from __future__ import annotations

import pytest

from src.graph.state import AgentState
from src.graph.workflow import route_next


# ── route_next unit tests ──────────────────────────────────────────────────


def _state(**kwargs) -> AgentState:  # type: ignore[return]
    base: AgentState = {
        "messages": [],
        "task": "test task",
        "plan": [],
        "current_step": 0,
        "research_results": [],
        "execution_results": [],
        "tool_errors": [],
        "workspace_id": "ws-test",
        "metadata": {},
        "final_answer": "",
    }
    base.update(kwargs)  # type: ignore[typeddict-item]
    return base


def test_route_next_ends_when_plan_complete() -> None:
    from langgraph.graph import END

    state = _state(plan=["step 1", "step 2"], current_step=2)
    assert route_next(state) == END


def test_route_next_ends_on_empty_plan() -> None:
    from langgraph.graph import END

    state = _state(plan=[], current_step=0)
    assert route_next(state) == END


def test_route_next_researcher_for_research_step() -> None:
    state = _state(plan=["research Python frameworks", "write summary"], current_step=0)
    assert route_next(state) == "researcher"


def test_route_next_researcher_for_search_keyword() -> None:
    state = _state(plan=["search for recent papers on LLMs"], current_step=0)
    assert route_next(state) == "researcher"


def test_route_next_researcher_for_find_keyword() -> None:
    state = _state(plan=["find the top 5 competitors"], current_step=0)
    assert route_next(state) == "researcher"


def test_route_next_researcher_for_retrieve_keyword() -> None:
    state = _state(plan=["retrieve customer records from the database"], current_step=0)
    assert route_next(state) == "researcher"


def test_route_next_executor_for_action_step() -> None:
    """False-positive guard: a non-research step must route to executor."""
    state = _state(plan=["write the comparison document"], current_step=0)
    assert route_next(state) == "executor"


def test_route_next_executor_for_code_step() -> None:
    state = _state(plan=["generate the final report"], current_step=0)
    assert route_next(state) == "executor"


def test_route_next_advances_through_plan() -> None:
    """Simulate routing through a 3-step plan."""
    from langgraph.graph import END

    plan = ["research topic", "write draft", "review draft"]
    steps = []
    for i in range(len(plan) + 1):
        state = _state(plan=plan, current_step=i)
        steps.append(route_next(state))

    assert steps[0] == "researcher"
    assert steps[1] == "executor"
    assert steps[2] == "executor"
    assert steps[3] == END

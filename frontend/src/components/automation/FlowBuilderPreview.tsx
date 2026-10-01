import { describeMergeDestination } from "@/lib/branchLabels";
import type { ReactNode } from "react";
import type { WorkflowWithStates } from "@/lib/pmTypes";
import type { FlowBuilderState } from "@/lib/flowBuilderTypes";
const triggerLabels: Record<string, string> = {
  "task.state_entered": "A task enters",
  "agent_run.approved": "An agent run is approved",
  "github.push": "A push arrives",
  "gitlab.push": "A push arrives",
  "github.pull_request_opened": "A pull request opens",
  "github.pull_request_merged": "A pull request merges",
  "github.pull_request_closed": "A pull request closes without merging",
  "github.pull_request_review_requested": "A review is requested",
  "gitlab.merge_request_opened": "A merge request opens",
  "gitlab.merge_request_merged": "A merge request merges",
  "gitlab.merge_request_closed": "A merge request closes without merging",
  "github.release_published": "A release is published",
  "gitlab.release_published": "A release is published",
  "github.check_suite_completed": "Checks finish",
  "gitlab.pipeline_completed": "A pipeline finishes",
};
function Prose({ children }: { children: string }) {
  return (
    <>
      {children
        .split(/(\*\*[^*]+\*\*)/g)
        .map((part, index) =>
          part.startsWith("**") && part.endsWith("**") ? (
            <strong key={index}>{part.slice(2, -2)}</strong>
          ) : (
            part
          ),
        )}
    </>
  );
}
const text = (value: unknown) => (typeof value === "string" ? value : "");
function scheduleLabel(value: unknown) {
  const schedule = text(value);
  const parts = schedule.split(" ");
  if (parts.length !== 5) return schedule;
  const [minute, hour, day, month, weekday] = parts;
  if (
    day === "*" &&
    month === "*" &&
    /^\d+$/.test(minute) &&
    /^\d+$/.test(hour)
  ) {
    const time = `${hour.padStart(2, "0")}:${minute.padStart(2, "0")} UTC`;
    if (weekday === "*") return `Every day at ${time}`;
    if (weekday === "1-5" || weekday === "1,2,3,4,5")
      return `Every weekday at ${time}`;
  }
  if (schedule === "0 * * * *") return "Every hour (UTC)";
  if (schedule === "0 0 * * 1") return "Every Monday at 00:00 UTC";
  const names: Record<string, string> = {
    "0": "Sunday",
    "1": "Monday",
    "2": "Tuesday",
    "3": "Wednesday",
    "4": "Thursday",
    "5": "Friday",
    "6": "Saturday",
    "7": "Sunday",
  };
  const describe = (field: string, labels?: Record<string, string>) =>
    field
      .split(",")
      .map((part) => {
        const [range, step] = part.split("/");
        const label =
          range === "*"
            ? "every value"
            : range
                .split("-")
                .map((value) => labels?.[value] ?? value)
                .join(" through ");
        return step
          ? `every ${step} starting from ${range === "*" ? "the beginning" : label}`
          : label;
      })
      .join(" or ");
  return `At ${minute === "*" ? "every minute" : `minute ${describe(minute)}`} ${hour === "*" ? "of every hour" : `of hour ${describe(hour)}`}${day !== "*" ? `, on day ${describe(day)} of the month` : ""}${month !== "*" ? `, in month ${describe(month)}` : ""}${weekday !== "*" ? `, on ${describe(weekday, names)}` : ""} (UTC)`;
}
export function FlowBuilderPreview({
  state,
  agents,
  workflows,
}: {
  state: FlowBuilderState;
  agents: Array<{ id: string; name: string }>;
  workflows: WorkflowWithStates[];
}) {
  const draft = state.draft;
  if (!draft) return null;
  const trigger = draft.trigger_config ?? {};
  const action = draft.action_config ?? {};
  const states = workflows.flatMap((workflow) => workflow.states);
  const stateName =
    states.find((state) => state.id === trigger.state_id)?.name ||
    text(trigger.state_type);
  const workflowName = workflows.find(
    (item) => item.workflow.id === draft.workflow_id,
  )?.workflow.name;
  const repository = text(trigger.repo_full_name);
  const agentName =
    state.agent?.name ??
    agents.find((agent) => agent.id === action.agent_id)?.name ??
    state.labels?.[text(action.agent_id)] ??
    "the selected agent";
  const targetName = state.labels?.[text(action.target_id)];
  const semanticCondition =
    text((trigger.semantic_condition as { text?: string } | undefined)?.text) ||
    draft.semantic_condition;
  const conditions: ReactNode[] = [];
  if (trigger.base_branch)
    conditions.push(
      <>
        the target branch is <strong>{text(trigger.base_branch)}</strong>
      </>,
    );
  if (trigger.branch)
    conditions.push(
      <>
        the branch is <strong>{text(trigger.branch)}</strong>
      </>,
    );
  if (trigger.tag_name)
    conditions.push(
      <>
        the tag is <strong>{text(trigger.tag_name)}</strong>
      </>,
    );
  if (trigger.tag_pattern)
    conditions.push(
      <>
        the tag matches <strong>{text(trigger.tag_pattern)}</strong>
      </>,
    );
  if (trigger.conclusion)
    conditions.push(
      <>
        the result is <strong>{text(trigger.conclusion)}</strong>
      </>,
    );
  if (Array.isArray(trigger.release_kinds) && trigger.release_kinds.length)
    conditions.push(
      <>
        the release is <strong>{trigger.release_kinds.join(" or ")}</strong>
      </>,
    );
  if (trigger.include_prerelease === false)
    conditions.push(
      <>
        the release is <strong>not a prerelease</strong>
      </>,
    );
  if (semanticCondition)
    conditions.push(
      <strong>
        {semanticCondition
          .replace(/[.!]$/, "")
          .replace(/^(The|A|An)\b/, (word) => word.toLowerCase())}
      </strong>,
    );
  const output = action.output as
    | { space_id?: string; collection_id?: string }
    | undefined;
  const schedule =
    trigger.schedule ||
    (
      {
        hourly: "0 * * * *",
        daily: "0 0 * * *",
        weekly: "0 0 * * 1",
      } as Record<string, string>
    )[
      text(trigger.preset) || text(trigger.category).replace("workspace_", "")
    ] ||
    trigger.category;
  const overrides = draft.agent_overrides;
  const agentSettings = state.creates_agent ? state.agent : undefined;
  const approvalMode = overrides?.approval_mode ?? agentSettings?.approval_mode;
  const approvalDescription: Record<string, string> = {
    never: "run and take actions without approval",
    risk_based: "ask for approval before sensitive or destructive actions",
    mutating_tools: "ask for approval before writes and actions",
    always: "ask for approval before each run and its actions",
    preset_default: "use its default approval policy",
  };
  const concurrentRuns =
    overrides?.max_concurrent_runs ?? agentSettings?.max_concurrent_runs;
  const tools = overrides?.allowed_tools ?? agentSettings?.allowed_tools ?? [];
  const skills = overrides?.skills ?? agentSettings?.skills ?? [];
  const targets =
    overrides?.allowed_targets ?? agentSettings?.allowed_targets ?? [];
  const agentInstructions = overrides?.system_prompt;
  const nextRun = state.next_runs?.[0];
  const instructions = state.template_key
    ? text(draft.template_inputs?.additional_instructions)
    : state.source_rule?.template_key &&
        action.additional_context ===
          state.source_rule.action_config?.additional_context
      ? ""
      : text(action.additional_context);
  const event = triggerLabels[draft.trigger_type] ?? draft.trigger_type;
  let actionDescription: ReactNode = (
    <>
      run <strong>{agentName}</strong>
      {targetName && targetName !== repository ? (
        <>
          {" "}
          on <strong>{targetName}</strong>
        </>
      ) : action.target_type === "workspace" ? (
        <>
          {" "}
          for <strong>this workspace</strong>
        </>
      ) : null}
    </>
  );
  if (draft.action_type === "move_to_state")
    actionDescription = (
      <>
        move the task to{" "}
        <strong>
          {states.find((state) => state.id === action.target_state_id)?.name ??
            "the selected state"}
        </strong>
      </>
    );
  if (draft.action_type === "merge_branch")
    actionDescription = (
      <>
        merge the branch into{" "}
        <strong>{describeMergeDestination(text(action.target_branch))}</strong>
      </>
    );

  return (
    <section
      aria-label={state.revision ? "Flow summary" : "Current flow description"}
      className="my-4 space-y-3 text-sm leading-relaxed"
    >
      <p>
        <strong>{draft.name}</strong> will {actionDescription}
        {draft.trigger_type === "cron" ? (
          <>
            {" "}
            on this schedule: <strong>{scheduleLabel(schedule)}</strong>
          </>
        ) : (
          <>
            {" "}
            when{" "}
            <strong>
              {event.charAt(0).toLowerCase() + event.slice(1)}
              {stateName ? ` ${stateName}` : ""}
            </strong>
            {repository || workflowName ? (
              <>
                {" "}
                in <strong>{repository || workflowName}</strong>
              </>
            ) : null}
          </>
        )}
        .
      </p>
      {draft.summary && (
        <p>
          <Prose>{draft.summary}</Prose>
        </p>
      )}
      {conditions.length > 0 && (
        <p>
          It runs only when{" "}
          {conditions.map((condition, index) => (
            <span key={index}>
              {index > 0 ? " and " : ""}
              {condition}
            </span>
          ))}
          .
        </p>
      )}
      {draft.team_id && (
        <p>
          It applies to{" "}
          <strong>
            {state.labels?.[draft.team_id] || "the selected team"}
          </strong>
          .
        </p>
      )}
      {output?.space_id && (
        <p>
          The result is saved as a document in{" "}
          <strong>
            {[
              state.labels?.[output.space_id] || "the selected space",
              output.collection_id
                ? state.labels?.[output.collection_id]
                : null,
            ]
              .filter(Boolean)
              .join(" / ")}
          </strong>
          .
        </p>
      )}
      {Boolean(action.base_branch || action.working_branch) && (
        <p>
          {action.base_branch ? (
            <>
              Runs use <strong>{text(action.base_branch)}</strong> as the base
              branch
              {action.working_branch ? (
                <>
                  {" "}
                  and <strong>{text(action.working_branch)}</strong> as the
                  working branch
                </>
              ) : null}
              .
            </>
          ) : (
            <>
              Runs use <strong>{text(action.working_branch)}</strong> as the
              working branch.
            </>
          )}
        </p>
      )}
      {state.agent && state.creates_agent && (
        <p>
          This also creates an agent named <strong>{state.agent.name}</strong>.
        </p>
      )}
      {draft.trigger_type === "cron" &&
        nextRun &&
        state.timezone &&
        state.timezone !== "UTC" && (
          <p>
            The next run is{" "}
            <strong>
              {new Intl.DateTimeFormat("en-GB", {
                timeZone: state.timezone,
                day: "numeric",
                month: "short",
                hour: "2-digit",
                minute: "2-digit",
                hourCycle: "h23",
              }).format(new Date(nextRun))}{" "}
              ({state.timezone})
            </strong>
            . The schedule stays fixed in UTC.
          </p>
        )}
      {approvalMode && approvalDescription[approvalMode] && (
        <p>
          The agent will <strong>{approvalDescription[approvalMode]}</strong>
          {concurrentRuns ? (
            <>
              {" "}
              and allow{" "}
              <strong>
                {concurrentRuns} concurrent{" "}
                {concurrentRuns === 1 ? "run" : "runs"}
              </strong>
            </>
          ) : null}
          .
        </p>
      )}
      {(tools.length > 0 ||
        skills.length > 0 ||
        targets.length > 0 ||
        overrides?.allowed_tools ||
        overrides?.skills) && (
        <details className="text-quiet-text-secondary">
          <summary className="cursor-pointer">Agent capabilities</summary>
          <p className="mt-2">
            It can work with{" "}
            <strong>
              {targets.map((value) => value.replaceAll("_", " ")).join(", ") ||
                "no working areas"}
            </strong>
            , use <strong>{tools.join(", ") || "no tools"}</strong>, and apply{" "}
            <strong>
              {skills
                .map(
                  (skill) =>
                    state.labels?.[skill.skill_id ?? skill.key ?? ""] ||
                    skill.key ||
                    "the selected skill",
                )
                .join(", ") || "no skills"}
            </strong>
            .
          </p>
          {agentInstructions && (
            <p className="mt-2 whitespace-pre-wrap">
              <Prose>{agentInstructions}</Prose>
            </p>
          )}
        </details>
      )}
      {draft.description && (
        <details className="text-quiet-text-secondary">
          <summary className="cursor-pointer">Description</summary>
          <p className="mt-2 whitespace-pre-wrap">
            <Prose>{draft.description}</Prose>
          </p>
        </details>
      )}
      <p>
        It {state.created_flow_id || !state.revision ? "is" : "will be"}{" "}
        <strong>{draft.paused ? "paused" : "active"}</strong>
        {state.revision && !state.created_flow_id ? " after you confirm" : ""}.
      </p>
      {instructions && (
        <details className="text-quiet-text-secondary">
          <summary className="cursor-pointer">Instructions</summary>
          <p className="mt-2 whitespace-pre-wrap">
            <Prose>{instructions}</Prose>
          </p>
        </details>
      )}
    </section>
  );
}

import type { AgentRunArtifact, AgentRunMessage, CodexAuthState, StructuredQuestion } from '@/lib/pmTypes';
export interface ParsedQuestions {
  questions: StructuredQuestion[];
  surroundingText: string;
}

export interface ParsedApprovalRequest {
  phase: string;
  title: string;
  summary: string;
  surroundingText: string;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string | null {
  return typeof value === 'string' && value.trim().length > 0 ? value.trim() : null;
}

function asOptionalString(value: unknown): string | undefined {
  return asString(value) ?? undefined;
}

function parseArtifactAssistantMessageSequenceNo(
  artifact: Pick<AgentRunArtifact, 'metadata'> | null | undefined,
): number | null {
  if (!artifact?.metadata || typeof artifact.metadata !== 'object' || Array.isArray(artifact.metadata)) {
    return null;
  }
  const value = artifact.metadata.assistant_message_sequence_no;
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

function parseStructuredQuestionsFromArtifact(
  message: Pick<AgentRunMessage, 'content' | 'sequence_no'>,
  artifacts: Array<Pick<AgentRunArtifact, 'artifact_type' | 'inline_content' | 'metadata'>>,
): ParsedQuestions | null {
  for (let index = artifacts.length - 1; index >= 0; index -= 1) {
    const artifact = artifacts[index];
    if (artifact.artifact_type !== 'human_input_request') continue;
    if (parseArtifactAssistantMessageSequenceNo(artifact) !== message.sequence_no) continue;
    if (!artifact.inline_content) continue;

    let payload: unknown;
    try {
      payload = JSON.parse(artifact.inline_content);
    } catch {
      continue;
    }

    const record = asRecord(payload);
    const rawQuestions = Array.isArray(record?.questions) ? record.questions : [];
    const questions: StructuredQuestion[] = rawQuestions.map((rawQuestion) => {
      const question = asRecord(rawQuestion);
      const rawOptions = Array.isArray(question?.options) ? question.options : [];
      return {
        id: asString(question?.id) ?? '',
        type: 'single_select' as const,
        text: asString(question?.text) ?? '',
        options: rawOptions.map((rawOption) => {
          const option = asRecord(rawOption);
          return {
            value: asString(option?.value) ?? '',
            label: asString(option?.label) ?? '',
            freetext: option?.freetext === true || undefined,
          };
        }).filter((option) => option.value.length > 0 && option.label.length > 0),
      };
    }).filter((question) => question.id.length > 0 && question.text.length > 0 && question.options.length > 0);

    if (questions.length === 0) continue;
    return {
      questions,
      surroundingText: message.content.trim(),
    };
  }

  return null;
}

function parseApprovalRequestFromArtifact(
  message: Pick<AgentRunMessage, 'content' | 'sequence_no'>,
  artifacts: Array<Pick<AgentRunArtifact, 'artifact_type' | 'inline_content' | 'metadata'>>,
): ParsedApprovalRequest | null {
  for (let index = artifacts.length - 1; index >= 0; index -= 1) {
    const artifact = artifacts[index];
    if (artifact.artifact_type !== 'human_approval_request') continue;
    if (parseArtifactAssistantMessageSequenceNo(artifact) !== message.sequence_no) continue;
    if (!artifact.inline_content) continue;

    let payload: unknown;
    try {
      payload = JSON.parse(artifact.inline_content);
    } catch {
      continue;
    }
    const input = asRecord(payload);
    const title = asString(input?.title);
    if (!title) continue;
    return {
      phase: asString(input?.phase)?.toLowerCase() ?? 'review',
      title,
      summary: asString(input?.summary) ?? '',
      surroundingText: message.content.trim(),
    };
  }

  return null;
}

export function parseMessageStructuredQuestions(
  message: Pick<AgentRunMessage, 'content' | 'sequence_no'>,
  artifacts: Array<Pick<AgentRunArtifact, 'artifact_type' | 'inline_content' | 'metadata'>> = [],
): ParsedQuestions | null {
  return parseStructuredQuestionsFromArtifact(message, artifacts);
}

export function parseMessageApprovalRequest(
  message: Pick<AgentRunMessage, 'content' | 'sequence_no'>,
  artifacts: Array<Pick<AgentRunArtifact, 'artifact_type' | 'inline_content' | 'metadata'>> = [],
): ParsedApprovalRequest | null {
  return parseApprovalRequestFromArtifact(message, artifacts);
}

export function parseLatestCodexAuthState(
  artifacts: Array<Pick<AgentRunArtifact, 'artifact_type' | 'inline_content' | 'created_at'>> = [],
): CodexAuthState | null {
  for (let index = artifacts.length - 1; index >= 0; index -= 1) {
    const artifact = artifacts[index];
    if (artifact.artifact_type !== 'codex_auth_state' || !artifact.inline_content) continue;

    let payload: unknown;
    try {
      payload = JSON.parse(artifact.inline_content);
    } catch {
      continue;
    }

    const record = asRecord(payload);
    const state = asString(record?.state);
    if (!record || !state) continue;

    return {
      provider: asOptionalString(record.provider),
      auth_mode: asOptionalString(record.auth_mode),
      state: state as CodexAuthState['state'],
      login_id: asOptionalString(record.login_id),
      auth_url: asOptionalString(record.auth_url),
      verification_url: asOptionalString(record.verification_url),
      user_code: asOptionalString(record.user_code),
      plan_type: asOptionalString(record.plan_type),
      error: asOptionalString(record.error),
      updated_at: asOptionalString(record.updated_at) ?? artifact.created_at,
    };
  }

  return null;
}

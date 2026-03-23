import type { AgentRunMessage, StructuredQuestion } from '@/lib/pmTypes';
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

function parseStructuredQuestionsFromToolInvocation(message: Pick<AgentRunMessage, 'content' | 'tool_invocations'>): ParsedQuestions | null {
  const invocations = Array.isArray(message.tool_invocations) ? message.tool_invocations : [];
  for (let index = invocations.length - 1; index >= 0; index -= 1) {
    const invocation = asRecord(invocations[index]);
    if (!invocation || invocation.tool_name !== 'request_human_input') continue;

    const input = asRecord(invocation.input);
    const rawQuestions = input?.questions;
    if (!Array.isArray(rawQuestions) || rawQuestions.length === 0) continue;

    const questions: StructuredQuestion[] = rawQuestions.map((rawQuestion) => {
      const question = asRecord(rawQuestion);
      const rawOptions = Array.isArray(question?.options) ? question?.options : [];
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

function parseApprovalRequestFromToolInvocation(message: Pick<AgentRunMessage, 'content' | 'tool_invocations'>): ParsedApprovalRequest | null {
  const invocations = Array.isArray(message.tool_invocations) ? message.tool_invocations : [];
  for (let index = invocations.length - 1; index >= 0; index -= 1) {
    const invocation = asRecord(invocations[index]);
    if (!invocation || invocation.tool_name !== 'request_human_approval') continue;

    const input = asRecord(invocation.input);
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

export function parseMessageStructuredQuestions(message: Pick<AgentRunMessage, 'content' | 'tool_invocations'>): ParsedQuestions | null {
  return parseStructuredQuestionsFromToolInvocation(message);
}

export function parseMessageApprovalRequest(message: Pick<AgentRunMessage, 'content' | 'tool_invocations'>): ParsedApprovalRequest | null {
  return parseApprovalRequestFromToolInvocation(message);
}

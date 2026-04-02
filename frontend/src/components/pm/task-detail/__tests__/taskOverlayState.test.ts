import { describe, expect, it } from 'vitest';

import { getTaskOverlayPresentationState } from '../taskOverlayState';
import type { StoryDetail, StoryRecurringSummary, WorkflowState } from '@/lib/pmTypes';

function makeStoryDetail(id: string): StoryDetail {
  return {
    story: {
      id,
    },
  } as StoryDetail;
}

function makeWorkflowState(id: string): WorkflowState {
  return {
    id,
  } as WorkflowState;
}

function makeRecurringSummary(templateId: string): StoryRecurringSummary {
  return {
    template_id: templateId,
  } as StoryRecurringSummary;
}

describe('getTaskOverlayPresentationState', () => {
  it('opens immediately and shows loading while the active story is still fetching', () => {
    expect(
      getTaskOverlayPresentationState('story-123', null),
    ).toEqual({
      open: true,
      loading: true,
      storyDetail: null,
      states: [],
      recurringSummary: null,
    });
  });

  it('keeps the overlay open and loading when cached data belongs to a different story', () => {
    expect(
      getTaskOverlayPresentationState('story-123', {
        storyId: 'story-999',
        storyDetail: makeStoryDetail('story-999'),
        states: [makeWorkflowState('done')],
        recurringSummary: makeRecurringSummary('template-999'),
      }),
    ).toEqual({
      open: true,
      loading: true,
      storyDetail: null,
      states: [],
      recurringSummary: null,
    });
  });

  it('shows the loaded story once the active story payload matches', () => {
    const loaded = {
      storyId: 'story-123',
      storyDetail: makeStoryDetail('story-123'),
      states: [makeWorkflowState('doing')],
      recurringSummary: makeRecurringSummary('template-123'),
    };

    expect(
      getTaskOverlayPresentationState('story-123', loaded),
    ).toEqual({
      open: true,
      loading: false,
      storyDetail: loaded.storyDetail,
      states: loaded.states,
      recurringSummary: loaded.recurringSummary,
    });
  });

  it('closes cleanly when there is no active story route', () => {
    expect(
      getTaskOverlayPresentationState(null, {
        storyId: 'story-123',
        storyDetail: makeStoryDetail('story-123'),
        states: [makeWorkflowState('doing')],
        recurringSummary: makeRecurringSummary('template-123'),
      }),
    ).toEqual({
      open: false,
      loading: false,
      storyDetail: null,
      states: [],
      recurringSummary: null,
    });
  });
});

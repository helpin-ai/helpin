import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  getActiveStoryRoute,
  getStoryOverlayCloseBehavior,
  matchStoryRoute,
  openStoryRoute,
  closeStoryRoute,
} from '../storyRouteNavigation';
import { useStoryPanelStore } from '@/stores/storyPanelStore';

describe('storyRouteNavigation', () => {
  beforeEach(() => {
    useStoryPanelStore.setState({
      storyId: null,
      requestKey: 0,
      lastClosedStoryId: null,
      lastClosedAt: 0,
    });
  });

  it('matches canonical story routes', () => {
    expect(matchStoryRoute('/w/test-docs/pm/stories/story-123')).toEqual({
      slug: 'test-docs',
      storyId: 'story-123',
    });
  });

  it('treats direct canonical story routes as active route entries', () => {
    expect(
      getActiveStoryRoute({
        pathname: '/w/test-docs/pm/stories/story-123',
      }),
    ).toEqual({
      slug: 'test-docs',
      storyId: 'story-123',
    });
  });

  it('treats non-story routes as contextual overlay entries', () => {
    expect(
      getStoryOverlayCloseBehavior({
        pathname: '/w/test-docs/pm/sprints',
      }, 'test-docs'),
    ).toEqual({
      type: 'close-panel',
    });
  });

  it('closes direct entries to the stories index fallback route', () => {
    expect(
      getStoryOverlayCloseBehavior({
        pathname: '/w/test-docs/pm/stories/story-123',
      }, 'test-docs'),
    ).toEqual({
      type: 'fallback-route',
      to: '/w/$slug/pm/stories/',
      params: { slug: 'test-docs' },
    });
  });

  it('opens contextual stories through the overlay store', () => {
    const navigate = vi.fn();

    openStoryRoute(
      navigate,
      {
        href: 'http://localhost:5173/w/test-docs/pm/sprints?team=abc',
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
      'story-123',
    );

    expect(navigate).not.toHaveBeenCalled();
    expect(useStoryPanelStore.getState().storyId).toBe('story-123');
  });

  it('navigates directly when already on a canonical story route', () => {
    const navigate = vi.fn();

    openStoryRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/stories/story-111',
      },
      'test-docs',
      'story-123',
    );

    expect(navigate).toHaveBeenCalledWith({
      to: '/w/$slug/pm/stories/$storyId',
      params: {
        slug: 'test-docs',
        storyId: 'story-123',
      },
    });
  });

  it('closes direct entries by navigating to the stories index fallback route', () => {
    const navigate = vi.fn();

    closeStoryRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/stories/story-123',
      },
      'test-docs',
    );

    expect(navigate).toHaveBeenCalledWith({
      to: '/w/$slug/pm/stories/',
      params: { slug: 'test-docs' },
    });
  });

  it('closes contextual overlays through the overlay store', () => {
    const navigate = vi.fn();
    useStoryPanelStore.setState({ storyId: 'story-123', requestKey: 1 });

    closeStoryRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
    );

    expect(navigate).not.toHaveBeenCalled();
    expect(useStoryPanelStore.getState().storyId).toBeNull();
  });

  it('does not immediately reopen the same contextual story right after close', () => {
    const navigate = vi.fn();
    useStoryPanelStore.setState({ storyId: 'story-123', requestKey: 1 });

    closeStoryRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
    );

    openStoryRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
      'story-123',
    );

    expect(useStoryPanelStore.getState().storyId).toBeNull();
  });

  it('still allows opening a different contextual story after close', () => {
    const navigate = vi.fn();
    useStoryPanelStore.setState({ storyId: 'story-123', requestKey: 1 });

    closeStoryRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
    );

    openStoryRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
      'story-456',
    );

    expect(useStoryPanelStore.getState().storyId).toBe('story-456');
  });
});

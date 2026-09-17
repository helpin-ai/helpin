import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/preact';
import { ComposeBar } from '../components/ComposeBar';

const attachment = { id: 'a', fileName: 'recording.mp4', fileType: 'video/mp4', fileSize: 1024, progress: 30 };
describe('attachment sending safeguards', () => {
  it.each(['uploading', 'error'] as const)('keeps text draft while attachment is %s', status => {
    const onSend = vi.fn();
    const { container } = render(<ComposeBar onSend={onSend} pendingAttachments={[{ ...attachment, status }]} />);
    const input = container.querySelector('textarea')!;
    fireEvent.input(input, { target: { value: 'See my recording' } });
    fireEvent.submit(container.querySelector('form')!);
    expect(onSend).not.toHaveBeenCalled();
    expect(input.value).toBe('See my recording');
  });
  it('offers retry and a meaningful error', () => {
    const retry = vi.fn();
    const { getByText } = render(<ComposeBar onSend={() => {}} onRetryAttachment={retry} pendingAttachments={[{ ...attachment, status: 'error', error: 'Upload failed. Try again.' }]} />);
    expect(getByText('Upload failed. Try again.')).toBeTruthy();
    fireEvent.click(getByText('Retry'));
    expect(retry).toHaveBeenCalledWith('a');
  });
});

import { useAttachmentUploads, MAX_SUPPORT_FILE_SIZE, type UploadAttachment } from '../hooks/useAttachmentUploads';
function Harness({ upload, files, scope = 'conversation' }: { upload: UploadAttachment; files: File[]; scope?: string }) {
  const state = useAttachmentUploads(scope, upload);
  return <><button onClick={() => state.select(files)}>Select</button><ComposeBar onSend={state.clear} pendingAttachments={state.pendingAttachments} onRetryAttachment={state.retry} onRemoveAttachment={state.remove} attachmentError={state.validationError} /></>;
}
function sizedFile(name: string, size: number, type = 'video/mp4') {
  const file = new File(['x'], name, { type });
  Object.defineProperty(file, 'size', { value: size });
  return file;
}

describe('widget attachment lifecycle', () => {
  it('allows exactly 100 MB and rejects larger files before upload', async () => {
    const upload = vi.fn().mockResolvedValue({ attachmentId: 'a', url: '/a' });
    const { getByText } = render(<Harness upload={upload} files={[sizedFile('allowed.mp4', MAX_SUPPORT_FILE_SIZE), sizedFile('large.mp4', MAX_SUPPORT_FILE_SIZE + 1)]} />);
    fireEvent.click(getByText('Select'));
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1));
    expect(getByText(/large.mp4 exceeds the 100 MB limit/)).toBeTruthy();
    await waitFor(() => expect(getByText(/Ready to send/)).toBeTruthy());
  });
  it('retains failed files for retry', async () => {
    const upload = vi.fn().mockRejectedValueOnce(new Error('Connection lost. Try again.')).mockResolvedValueOnce({ attachmentId: 'a', url: '/a' });
    const { getByText } = render(<Harness upload={upload} files={[sizedFile('recording.mp4', 100)]} />);
    fireEvent.click(getByText('Select'));
    await waitFor(() => expect(getByText('Connection lost. Try again.')).toBeTruthy());
    fireEvent.click(getByText('Retry'));
    await waitFor(() => expect(getByText(/Ready to send/)).toBeTruthy());
    expect(upload).toHaveBeenCalledTimes(2);
  });
  it('shows progress and cancels removed uploads without reviving them', async () => {
    let complete!: (result: { attachmentId: string; url: string }) => void;
    const upload = vi.fn((_file, _id, options) => { options.onProgress(42); return new Promise<{ attachmentId: string; url: string }>(resolve => { complete = resolve; }); });
    const { getByText, getByLabelText, queryByText } = render(<Harness upload={upload} files={[sizedFile('recording.mp4', 100)]} />);
    fireEvent.click(getByText('Select'));
    await waitFor(() => expect(getByText(/Uploading 42%/)).toBeTruthy());
    fireEvent.click(getByLabelText('Cancel upload of recording.mp4'));
    expect(upload.mock.calls[0][2].signal.aborted).toBe(true);
    complete({ attachmentId: 'a', url: '/a' });
    await waitFor(() => expect(queryByText('recording.mp4')).toBeNull());
  });
  it('registers queued files and cancels the batch on conversation change', async () => {
    let complete!: (result: { attachmentId: string; url: string }) => void;
    const upload = vi.fn(() => new Promise<{ attachmentId: string; url: string }>(resolve => { complete = resolve; }));
    const files = [sizedFile('first.mp4', 100), sizedFile('queued.mp4', 100)];
    const { getByText, rerender, queryByText } = render(<Harness upload={upload} files={files} />);
    fireEvent.click(getByText('Select'));
    expect(getByText('queued.mp4')).toBeTruthy();
    rerender(<Harness upload={upload} files={files} scope="other-conversation" />);
    await waitFor(() => expect(queryByText('queued.mp4')).toBeNull());
    complete({ attachmentId: 'a', url: '/a' });
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1));
  });
});

 it('preserves uploads when a new conversation receives its server ID', async () => {
    const upload = vi.fn(() => new Promise<{ attachmentId: string; url: string }>(() => {}));
    const files = [sizedFile('recording.mp4', 100)];
    const { getByText, rerender } = render(<Harness upload={upload} files={files} scope="__new__" />);
    fireEvent.click(getByText('Select'));
    rerender(<Harness upload={upload} files={files} scope="server-id" />);
    await waitFor(() => expect(getByText('recording.mp4')).toBeTruthy());
    expect(upload).toHaveBeenCalledTimes(1);
  });

import { MessageBubble } from '../components/MessageBubble';
it('renders video controls with a download fallback', () => {
  const { container, getByText } = render(<MessageBubble message={{ id: 'm', conversationId: 'c', role: 'customer', content: '', isInternal: false, createdAt: '2026-09-10T00:00:00Z', attachments: [{ id: 'a', fileName: 'recording.mp4', fileType: 'video/mp4', fileSize: 100, url: 'https://files.example/recording.mp4' }] }} />);
  const video = container.querySelector('video')!;
  expect(video.controls).toBe(true);
  expect(video.preload).toBe('metadata');
  expect(container.querySelector('a[download="recording.mp4"]')).toBeTruthy();
  fireEvent.error(video);
  expect(getByText(/cannot play in your browser/)).toBeTruthy();
  expect(container.querySelector('a[download="recording.mp4"]')).toBeTruthy();
});

describe('attachment edge-case regressions', () => {
  it('rejects empty files without calling the upload transport', async () => {
    const upload = vi.fn();
    const { getByText } = render(<Harness upload={upload} files={[sizedFile('empty.txt', 0, 'text/plain')]} />);
    fireEvent.click(getByText('Select'));
    await waitFor(() => expect(getByText(/empty.txt is empty/)).toBeTruthy());
    expect(upload).not.toHaveBeenCalled();
  });
  it('aborts an in-flight upload when the widget unmounts', async () => {
    const upload = vi.fn(() => new Promise<{ attachmentId: string; url: string }>(() => {}));
    const { getByText, unmount } = render(<Harness upload={upload} files={[sizedFile('video.mp4', 100)]} />);
    fireEvent.click(getByText('Select'));
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1));
    const signal = (upload.mock.calls as unknown as Array<[File, string, { signal: AbortSignal }]>)[0][2].signal;
    unmount();
    expect(signal.aborted).toBe(true);
  });
  it('continues a batch after a failed file while retaining it for retry', async () => {
    const upload = vi.fn().mockRejectedValueOnce(new Error('Connection lost')).mockResolvedValueOnce({ attachmentId: 'second', url: '/second' });
    const { getByText } = render(<Harness upload={upload} files={[sizedFile('first.mp4', 100), sizedFile('second.mp4', 100)]} />);
    fireEvent.click(getByText('Select'));
    await waitFor(() => expect(getByText(/Ready to send/)).toBeTruthy());
    expect(getByText('Connection lost')).toBeTruthy();
    expect(upload).toHaveBeenCalledTimes(2);
  });
  it('recovers from an upload attempted before the chat connects', async () => {
    const upload = vi.fn().mockRejectedValueOnce(new Error('Chat is not connected yet. Please wait and retry the upload.')).mockResolvedValueOnce({ attachmentId: 'connected', url: '/connected' });
    const { getByText } = render(<Harness upload={upload} files={[sizedFile('photo.png', 100, 'application/octet-stream')]} />);
    fireEvent.click(getByText('Select'));
    await waitFor(() => expect(getByText(/Chat is not connected yet/)).toBeTruthy());
    fireEvent.click(getByText('Retry'));
    await waitFor(() => expect(getByText(/Ready to send/)).toBeTruthy());
    expect(upload.mock.calls[1][0]).toBe(upload.mock.calls[0][0]);
  });
});

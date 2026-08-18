type ClipboardFileSource = {
  files?: ArrayLike<File> | null;
};

export function getClipboardImageFiles(data: ClipboardFileSource | null | undefined) {
  return Array.from(data?.files ?? []).filter((file) => file.type.startsWith('image/'));
}

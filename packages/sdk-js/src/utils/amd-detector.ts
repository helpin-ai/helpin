// src/utils/amd-detector.ts
export function isAMDEnvironment(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof (window as any).define === 'function' &&
    (window as any).define.amd
  );
}

export function getAMDDefine(): any {
  return isAMDEnvironment() ? (window as any).define : undefined;
}

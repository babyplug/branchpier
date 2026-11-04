import { Call } from '@wailsio/runtime';

export type Change = {
  id: string;
  path: string;
  original?: string;
  index: string;
  worktree: string;
  untracked: boolean;
  conflicted: boolean;
};

export type Target = {
  remote: string;
  branch: string;
  url: string;
  webUrl: string;
};

export type State = {
  path: string;
  name: string;
  branch: string;
  head: string;
  upstream: string;
  ahead: number;
  behind: number;
  files: Change[];
  branches: string[] | null;
  target: Target | null;
};

export type Commit = {
  hash: string;
  parents: string[] | null;
  subject: string;
  author: string;
  date: string;
  refs: string;
};

export type SyncAction = 'fetch' | 'pull' | 'push';

function invoke<T>(method: string, ...args: unknown[]): Promise<T> {
  return Call.ByName('main.GitService.' + method, ...args);
}

export const git = {
  open: () => invoke<State | null>('Open'),
  clone: (remote: string) => invoke<State | null>('Clone', remote),
  status: (path: string) => invoke<State>('Status', path),
  diff: (path: string, file: string, staged: boolean) => invoke<string>('Diff', path, file, staged),
  stage: (path: string, file: string, staged: boolean) => invoke<void>('SetStaged', path, file, staged),
  commit: (path: string, message: string) => invoke<void>('Commit', path, message),
  switch: (path: string, branch: string, create: boolean) => invoke<void>('Switch', path, branch, create),
  sync: (state: State, action: SyncAction) => invoke<void>('Sync', state.path, action, state.branch, state.head, state.target),
  history: (path: string) => invoke<Commit[]>('History', path),
  commitDiff: (path: string, hash: string) => invoke<string>('CommitDiff', path, hash),
  openRemote: (path: string) => invoke<void>('OpenRemote', path),
};

import { useEffect, useMemo, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { createRoot } from 'react-dom/client';
import { git } from './git';
import type { Change, Commit, State, SyncAction } from './git';
import { laneColor, layoutGraph } from './graph';
import type { GraphRow } from './graph';
import './style.css';

type Selection = { id: string; staged: boolean };

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function App() {
  const [state, setState] = useState<State | null>(null);
  const [view, setView] = useState<'changes' | 'history'>('changes');
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [cloneOpen, setCloneOpen] = useState(false);
  const [cloneURL, setCloneURL] = useState('');
  const [branchOpen, setBranchOpen] = useState(false);
  const [branchName, setBranchName] = useState('');
  const [message, setMessage] = useState('');
  const inFlight = useRef(false);

  async function act(label: string, operation: () => Promise<void>) {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(label);
    setNotice('');
    setError('');
    try {
      await operation();
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      inFlight.current = false;
      setBusy('');
    }
  }

  async function choose(clone = false) {
    await act(clone ? 'Cloning repository…' : 'Opening repository…', async () => {
      const next = clone ? await git.clone(cloneURL) : await git.open();
      if (!next) return;
      setState(next);
      setView('changes');
      setMessage('');
      setBranchOpen(false);
      setBranchName('');
      setCloneOpen(false);
      setCloneURL('');
    });
  }

  async function change(label: string, operation: () => Promise<void>, success?: string) {
    if (!state) return;
    await act(label, async () => {
      try {
        await operation();
      } catch (cause) {
        const next = await git.status(state.path).catch(() => null);
        if (next) setState(next);
        throw cause;
      }
      setState(await git.status(state.path));
      if (success) setNotice(success);
    });
  }

  function sync(action: SyncAction) {
    if (!state?.target) return;
    const destination = state.target.remote + '/' + state.target.branch;
    const labels = { fetch: 'Fetching…', pull: 'Pulling…', push: 'Pushing…' };
    const done = { fetch: 'Fetched ' + state.target.remote + '.', pull: 'Pulled ' + destination + '.', push: 'Pushed to ' + destination + '.' };
    void change(labels[action], () => git.sync(state, action), done[action]);
  }

  function commit(event: FormEvent) {
    event.preventDefault();
    if (!state || !message.trim()) return;
    void change('Committing…', async () => {
      await git.commit(state.path, message);
      setMessage('');
    }, 'Commit created.');
  }

  function createBranch(event: FormEvent) {
    event.preventDefault();
    if (!state || !branchName.trim()) return;
    void change('Creating branch…', async () => {
      await git.switch(state.path, branchName.trim(), true);
      setBranchOpen(false);
      setBranchName('');
    });
  }

  const stagedCount = state?.files.filter(file => file.index !== '.' && !file.conflicted).length ?? 0;
  const conflicted = state?.files.some(file => file.conflicted) ?? false;
  const target = state?.target;
  const disabled = Boolean(busy);
  const pushBlocked = Boolean(target && target.branch !== state?.branch);
  return (
    <main className="app">
      <header className="toolbar">
        <div className="wordmark"><BranchIcon /> BranchPier</div>
        <div className="toolbar-actions">
          <button disabled={disabled} onClick={() => void choose()}>Open repository</button>
          <button disabled={disabled} aria-expanded={cloneOpen} onClick={() => setCloneOpen(!cloneOpen)}>Clone</button>
        </div>
      </header>
      {cloneOpen && (
        <form className="inline-form" onSubmit={event => { event.preventDefault(); void choose(true); }}>
          <label htmlFor="clone-url">Clone URL</label>
          <input id="clone-url" value={cloneURL} onChange={event => setCloneURL(event.target.value)}
            placeholder="git@gitlab.example.com:team/repo.git" disabled={disabled} autoFocus />
          <button type="submit" className="primary" disabled={disabled || !cloneURL.trim()}>Choose folder</button>
        </form>
      )}

      {state && (
        <>
          <section className="repo-bar" aria-label="Repository and branch">
            <div className="repo-name">
              <strong>{state.name}</strong>
              <span title={state.path}>{state.path}</span>
            </div>
            <div className="branch-control">
              <label htmlFor="branch">Branch</label>
              <select id="branch" value={state.branch} disabled={disabled} onChange={event => {
                const branch = event.target.value;
                if (branch) void change('Switching branch…', () => git.switch(state.path, branch, false));
              }}>
                {!state.branch && <option value="">Detached HEAD</option>}
                {state.branch && !state.branches?.includes(state.branch) && <option value={state.branch}>{state.branch}</option>}
                {state.branches?.map(branch => <option value={branch} key={branch}>{branch}</option>)}
              </select>
              <button disabled={disabled || !state.head} aria-expanded={branchOpen} onClick={() => setBranchOpen(!branchOpen)}>New branch</button>
            </div>
            <div className="sync">
              <div className="sync-target" title={target?.url}>
                <strong>{target ? target.remote + '/' + target.branch : 'No sync target'}</strong>
                {state.upstream && <span aria-label={state.ahead + ' ahead, ' + state.behind + ' behind'}>↑ {state.ahead} &nbsp; ↓ {state.behind}</span>}
              </div>
              <div className="toolbar-actions">
                <button disabled={disabled || !target} onClick={() => sync('fetch')}>Fetch</button>
                <button disabled={disabled || !target || !state.upstream} onClick={() => sync('pull')}>Pull</button>
                <button className="primary" disabled={disabled || !target || !state.head || pushBlocked}
                  title={pushBlocked ? 'The upstream branch has a different name. Push explicitly in Git.' : undefined}
                  onClick={() => sync('push')}>
                  {state.upstream ? 'Push' : 'Publish branch'}
                </button>
              </div>
            </div>
          </section>
          {branchOpen && (
            <form className="inline-form" onSubmit={createBranch}>
              <label htmlFor="branch-name">New branch</label>
              <input id="branch-name" placeholder="feature/my-change" value={branchName} onChange={event => setBranchName(event.target.value)}
                disabled={disabled} autoFocus />
              <button className="primary" disabled={disabled || !branchName.trim()}>Create and switch</button>
            </form>
          )}
        </>
      )}

      {(busy || notice) && <div className="notice" role="status">{busy || notice}</div>}
      {error && <div className="notice error" role="alert"><span>{error}</span><button onClick={() => setError('')}>Dismiss</button></div>}

      {!state ? (
        <section className="welcome">
          <div className="welcome-icon"><BranchIcon /></div>
          <p className="eyebrow">Your Git workspace</p>
          <h1>Open a repository</h1>
          <p>Review changes, follow branches, and push to your remote.</p>
          <button className="primary" disabled={disabled} onClick={() => void choose()}>Choose a local folder</button>
        </section>
      ) : (
        <>
          <nav className="views" aria-label="Repository view">
            <div className="view-tabs">
              <button aria-pressed={view === 'changes'} onClick={() => setView('changes')}>Changes <span>{state.files.length}</span></button>
              <button aria-pressed={view === 'history'} onClick={() => setView('history')}>History</button>
            </div>
            <div className="toolbar-actions">
              {target?.webUrl && <button disabled={disabled} onClick={() => void act('Opening remote…', () => git.openRemote(state.path))}>Open remote in browser</button>}
              <button disabled={disabled} onClick={() => void act('Refreshing…', async () => setState(await git.status(state.path)))}>Refresh</button>
            </div>
          </nav>
          {view === 'changes' ? (
            <Changes key={state.path} state={state} disabled={disabled}
              onStage={(id, staged) => void change(staged ? 'Staging…' : 'Unstaging…', () => git.stage(state.path, id, staged))}
              onCommit={commit} message={message} setMessage={setMessage} stagedCount={stagedCount} conflicted={conflicted} />
          ) : <History key={state.path} state={state} />}
        </>
      )}
    </main>
  );
}

function BranchIcon() {
  return (
    <svg width="23" height="23" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M6 7v10m12-10a5 5 0 0 1-5 5H6" stroke="currentColor" strokeWidth="1.8" />
      <circle cx="6" cy="5" r="2" stroke="currentColor" strokeWidth="1.8" />
      <circle cx="6" cy="19" r="2" stroke="currentColor" strokeWidth="1.8" />
      <circle cx="18" cy="5" r="2" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  );
}

function Changes({ state, disabled, onStage, onCommit, message, setMessage, stagedCount, conflicted }: {
  state: State;
  disabled: boolean;
  onStage: (id: string, staged: boolean) => void;
  onCommit: (event: FormEvent) => void;
  message: string;
  setMessage: (message: string) => void;
  stagedCount: number;
  conflicted: boolean;
}) {
  const unstaged = state.files.filter(file => file.worktree !== '.' || file.conflicted);
  const staged = state.files.filter(file => file.index !== '.' && !file.conflicted);
  const [selection, setSelection] = useState<Selection | null>(null);
  const selected = (selection?.staged ? staged : unstaged).find(file => file.id === selection?.id);
  const file = selected ?? unstaged[0] ?? staged[0];
  const isStaged = selected ? Boolean(selection?.staged) : unstaged.length === 0;
  const id = file?.id;
  const [diff, setDiff] = useState('');
  const [diffError, setDiffError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setDiff('');
    setDiffError('');
    setLoading(Boolean(id));
    if (!id) return;
    let current = true;
    git.diff(state.path, id, isStaged).then(
      result => { if (current) setDiff(result); },
      cause => { if (current) setDiffError(errorMessage(cause)); },
    ).finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [state, id, isStaged]);

  function files(list: Change[], stagedGroup: boolean) {
    return list.map(change => (
      <button key={change.id} className="file-row"
        aria-pressed={id === change.id && isStaged === stagedGroup}
        onClick={() => setSelection({ id: change.id, staged: stagedGroup })}>
        <span className={'file-status ' + (change.conflicted ? 'conflict' : stagedGroup ? 'staged' : '')}>
          {change.conflicted ? '!' : stagedGroup ? change.index : change.worktree}
        </span>
        <span className="filename" title={change.path}>{change.path}</span>
      </button>
    ));
  }

  return (
    <section className="workspace changes">
      <aside className="sidebar">
        <div className="files">
          <h2>Unstaged <span>{unstaged.length}</span></h2>
          {files(unstaged, false)}
          <h2>Staged <span>{staged.length}</span></h2>
          {files(staged, true)}
          {state.files.length === 0 && <p className="hint">Working tree is clean.</p>}
        </div>
        <form className="commit-form" onSubmit={onCommit}>
          <label htmlFor="commit-message">Commit message</label>
          <textarea id="commit-message" value={message} disabled={disabled} onChange={event => setMessage(event.target.value)}
            placeholder="Summarize your changes" rows={3} />
          {conflicted && <p className="hint">Resolve conflicts before committing.</p>}
          <button className="primary" disabled={disabled || !stagedCount || conflicted || !message.trim()}>
            Commit {stagedCount ? stagedCount + (stagedCount === 1 ? ' file' : ' files') : ''}
          </button>
        </form>
      </aside>
      <article className="detail">
        {file ? (
          <>
            <header className="detail-header">
              <div>
                <p className="eyebrow">{file.conflicted ? 'Conflict' : isStaged ? 'Staged diff' : 'Unstaged diff'}</p>
                <h2>{file.path}</h2>
                {file.original && <p className="hint">From {file.original}</p>}
              </div>
              <button disabled={disabled || file.conflicted} onClick={() => onStage(file.id, !isStaged)}>
                {isStaged ? 'Unstage file' : 'Stage file'}
              </button>
            </header>
            <Diff text={diff} loading={loading} error={diffError} />
          </>
        ) : <p className="hint empty-detail">File changes will appear here.</p>}
      </article>
    </section>
  );
}

function History({ state }: { state: State }) {
  const [commits, setCommits] = useState<Commit[]>([]);
  const [selected, setSelected] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [diff, setDiff] = useState('');
  const [diffError, setDiffError] = useState('');
  const [diffLoading, setDiffLoading] = useState(false);
  const graph = useMemo(() => layoutGraph(commits), [commits]);
  const commit = commits.find(item => item.hash === selected) ?? commits[0];
  const hash = commit?.hash;

  useEffect(() => {
    let current = true;
    setLoading(true);
    setError('');
    git.history(state.path).then(
      result => { if (current) setCommits(result); },
      cause => { if (current) setError(errorMessage(cause)); },
    ).finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [state]);

  useEffect(() => {
    setDiff('');
    setDiffError('');
    setDiffLoading(Boolean(hash));
    if (!hash) return;
    let current = true;
    git.commitDiff(state.path, hash).then(
      result => { if (current) setDiff(result); },
      cause => { if (current) setDiffError(errorMessage(cause)); },
    ).finally(() => { if (current) setDiffLoading(false); });
    return () => { current = false; };
  }, [state.path, hash]);

  return (
    <section className="workspace history">
      <div className="commit-list" aria-label="Commit history">
        <div className="list-heading">
          <h2>Commit graph</h2>
          <span>{loading ? 'Loading…' : commits.length === 150 ? 'Latest 150 commits' : commits.length + ' commits'}</span>
        </div>
        {error && <p className="hint" role="alert">{error}</p>}
        {!loading && !error && !commits.length && <p className="hint">No commits yet.</p>}
        {commits.map((item, index) => (
          <button className="commit-row" key={item.hash} aria-pressed={item.hash === hash} onClick={() => setSelected(item.hash)}>
            <GraphCell row={graph.rows[index]} width={graph.width} />
            <span className="commit-row-text">
              <strong>{item.subject || '(no subject)'}</strong>
              <span>{item.refs || item.author}</span>
            </span>
            <code>{item.hash.slice(0, 7)}</code>
          </button>
        ))}
      </div>
      <article className="detail">
        {commit && (
          <>
            <header className="detail-header">
              <div>
                <p className="eyebrow">Commit {commit.hash.slice(0, 12)}</p>
                <h2>{commit.subject || '(no subject)'}</h2>
                <p className="hint">{commit.author} · {commit.date}</p>
                {commit.parents && <p className="hint">Parents {commit.parents.map(parent => parent.slice(0, 7)).join(', ')}</p>}
              </div>
            </header>
            <Diff text={diff} loading={diffLoading} error={diffError} />
          </>
        )}
      </article>
    </section>
  );
}

function GraphCell({ row, width }: { row: GraphRow; width: number }) {
  const x = (lane: number) => 12 + lane * 14;
  const height = 54;
  return (
    <svg className="graph" aria-hidden="true" width={Math.min(width, 140)} height={height} viewBox={'0 0 ' + width + ' ' + height} preserveAspectRatio="none">
      {row.straight.map(edge => <path key={edge.from} d={'M' + x(edge.from) + ' 0 V' + height} stroke={laneColor(edge.color)} />)}
      {row.incoming && <path d={'M' + x(row.lane) + ' 0 V27'} stroke={laneColor(row.color)} />}
      {row.parents.map((edge, index) => (
        <path key={index} d={`M${x(edge.from)} 27 C${x(edge.from)} 44 ${x(edge.to)} 39 ${x(edge.to)} ${height}`}
          stroke={laneColor(edge.color)} />
      ))}
      <circle cx={x(row.lane)} cy="27" r="4.5" stroke={laneColor(row.color)} />
    </svg>
  );
}

function Diff({ text, loading, error }: { text: string; loading: boolean; error: string }) {
  if (loading) return <p className="hint empty-detail" role="status">Loading diff…</p>;
  if (error) return <p className="hint empty-detail" role="alert">{error}</p>;
  if (!text) return <p className="hint empty-detail">No text diff to display.</p>;
  const lines = text.slice(0, 240_000).trimEnd().split('\n', 2001);
  return (
    <div className="diff">
      <pre><code>{lines.slice(0, 2000).map((line, index) => {
        const style = line.startsWith('+++ ') || line.startsWith('--- ') || line.startsWith('@@') ? 'hunk'
          : line.startsWith('+') ? 'addition' : line.startsWith('-') ? 'deletion' : '';
        return <span key={index} className={style}>{line + '\n'}</span>;
      })}</code></pre>
      {(text.length > 240_000 || lines.length > 2000) && <p className="hint">Diff preview truncated.</p>}
    </div>
  );
}

createRoot(document.getElementById('root')!).render(<App />);

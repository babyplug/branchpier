import type { Commit } from './git';

type Lane = { hash: string; color: number };
type Edge = { from: number; to: number; color: number };
export type GraphRow = {
  lane: number;
  color: number;
  incoming: boolean;
  straight: Edge[];
  parents: Edge[];
};

const palette = ['#69e4d0', '#b8a4ff', '#ffb677', '#79b8ff', '#f08fbd', '#cad977'];
export const laneColor = (color: number) => palette[color % palette.length];

export function layoutGraph(commits: Commit[]) {
  const visible = new Set(commits.map(commit => commit.hash));
  const lanes: (Lane | null)[] = [];
  let laneCount = 1;
  let nextColor = 0;

  const put = (lane: Lane) => {
    const gap = lanes.indexOf(null);
    const index = gap === -1 ? lanes.length : gap;
    lanes[index] = lane;
    laneCount = Math.max(laneCount, lanes.length);
    return index;
  };

  const rows = commits.map<GraphRow>(commit => {
    const existing = lanes.findIndex(lane => lane?.hash === commit.hash);
    const lane = existing >= 0 ? existing : put({ hash: commit.hash, color: nextColor++ });
    const color = lanes[lane]!.color;
    const straight = lanes.flatMap((item, index) => item && index !== lane
      ? [{ from: index, to: index, color: item.color }]
      : []);

    lanes[lane] = null;
    const parents = (commit.parents ?? []).filter(hash => visible.has(hash)).map((hash, index) => {
      const known = lanes.findIndex(item => item?.hash === hash);
      if (known >= 0) return { from: lane, to: known, color: lanes[known]!.color };
      const parent = { hash, color: index === 0 ? color : nextColor++ };
      if (index === 0) {
        lanes[lane] = parent;
        return { from: lane, to: lane, color };
      }
      return { from: lane, to: put(parent), color: parent.color };
    });

    while (lanes.length && lanes.at(-1) === null) lanes.pop();
    return { lane, color, incoming: existing >= 0, straight, parents };
  });

  return { rows, width: 22 + laneCount * 14 };
}

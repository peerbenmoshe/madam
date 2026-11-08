// SPIKE ONLY (branch spike/graph, never merged). The graph from spec-v1.md §9
// drawn with Recharts over fake data, to answer docs/build-order.md §3.
import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactElement,
  type ReactNode,
} from "react";
import {
  Line,
  LineChart,
  ReferenceLine,
  Tooltip,
  XAxis,
  YAxis,
  type DotItemDotProps,
  type LabelProps,
  type MouseHandlerDataParam,
  type XAxisTickContentProps,
} from "recharts";
import { getViolations, provokeViolation, subscribe } from "./cspLog";
import {
  ALL_DAYS,
  CHECKLIST,
  CYCLE_STARTS,
  FIXED_MINUTES,
  type Day,
} from "./fakeData";
import { lineSegments } from "./segments";

// Colours handed to Recharts as SVG attributes: the @theme tokens, by name.
const INK = "var(--color-ink)";
const MUTED = "var(--color-muted)";
// The tokens' computed values, for the colour audit.
const TOKEN_RGB: Record<string, string> = {
  "rgb(255, 105, 180)": "accent",
  "rgb(26, 26, 26)": "ink",
  "rgb(89, 89, 89)": "muted",
  "rgb(255, 255, 255)": "surface",
};

type Unit = "C" | "F";
type TapMode = "dot" | "column";
type Kind = "dot" | "ghost" | "column";

// One row per calendar day. s0, s1, … hold the value only on the rows of
// that segment, so each <Line dataKey="sK"> draws one run.
type Row = {
  date: string;
  day: Day;
  y: number | null; // the solid dot (adjusted value)
  ghost: number | null; // raw value, only when adjusted and ghosts are on
  hitY: number | null;
  hitGhost: number | null;
  [segment: `s${number}`]: number | undefined;
};

const RANGES = [7, 14, 30, 90] as const;

// URL switches, so a headless browser can load any state without clicks:
// ?noanim&range=90&unit=F&ghosts&unknown&tap=column&provoke
const params = new URLSearchParams(window.location.search);
const ANIMATE = !params.has("noanim");
const INITIAL_RANGE =
  RANGES.find((r) => r === Number(params.get("range"))) ?? 14;
const CHART_HEIGHT = 280;
const Y_AXIS_WIDTH = 44;
const X_PADDING = 10;
const MARGIN = { top: 18, right: 8, bottom: 4, left: 0 };

function fmtDate(date: string): string {
  const [, m, d] = date.split("-");
  return `${Number(d)}/${Number(m)}`; // assumption: d/m, the spec names none
}

function value(day: Day, unit: Unit, which: "adj" | "raw"): number | null {
  const h =
    unit === "C"
      ? which === "adj"
        ? day.adjC
        : day.rawC
      : which === "adj"
        ? day.adjF
        : day.rawF;
  return h === null ? null : h / 100;
}

function hhmm(min: number): string {
  return `${String(Math.floor(min / 60)).padStart(2, "0")}:${String(min % 60).padStart(2, "0")}`;
}

function useWidth(ref: React.RefObject<HTMLElement | null>): number {
  const [w, setW] = useState(0);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setW(el.clientWidth));
    ro.observe(el);
    setW(el.clientWidth);
    return () => ro.disconnect();
  }, [ref]);
  return w;
}

type Tap = { target: string; result: string; ok: boolean };

export default function GraphSpike() {
  const [range, setRange] = useState<(typeof RANGES)[number]>(INITIAL_RANGE);
  const [unit, setUnit] = useState<Unit>(
    params.get("unit") === "F" ? "F" : "C",
  );
  const [ghosts, setGhosts] = useState(params.has("ghosts"));
  const [unknownStart, setUnknownStart] = useState(params.has("unknown"));
  const [tapMode, setTapMode] = useState<TapMode>(
    params.get("tap") === "column" ? "column" : "dot",
  );
  useEffect(() => {
    if (params.has("provoke")) provokeViolation();
  }, []);
  const [popup, setPopup] = useState<{ day: Day; kind: Kind } | null>(null);
  const [taps, setTaps] = useState<Tap[]>([]);
  const [target, setTarget] = useState("2026-10-31");
  const [audit, setAudit] = useState<string[]>([]);
  const violationCount = useSyncExternalStore(
    subscribe,
    () => getViolations().length,
  );

  const box = useRef<HTMLDivElement>(null);
  const width = useWidth(box);
  // Set by a dot's or the chart's click handler, read by the wrapper's
  // handler, which runs after them (bubbling): so a tap that hits nothing is
  // logged too.
  const hit = useRef<{ day: Day; kind: Kind } | null>(null);

  const days = useMemo(() => ALL_DAYS.slice(-range), [range]);
  const runs = useMemo(
    () =>
      lineSegments(
        days.map((d) => ({
          hasTemp: d.rawMilli !== null,
          disturbed: d.disturbances.length > 0,
        })),
      ),
    [days],
  );

  const rows: Row[] = useMemo(() => {
    const segOf = new Map<number, number>();
    runs.forEach((run, k) => run.forEach((i) => segOf.set(i, k)));
    return days.map((day, i) => {
      const y = value(day, unit, "adj");
      const ghost = ghosts && day.adjusted ? value(day, unit, "raw") : null;
      const row: Row = {
        date: day.date,
        day,
        y,
        ghost,
        hitY: y,
        hitGhost: ghost,
      };
      const k = segOf.get(i);
      if (k !== undefined && y !== null) row[`s${k}`] = y;
      return row;
    });
  }, [days, runs, unit, ghosts]);

  // Y range from every value in the window, ghosts included even when hidden,
  // so toggling ghosts doesn't move the axis.
  const { domain, ticks } = useMemo(() => {
    const step = unit === "C" ? 0.1 : 0.2;
    const vals = days.flatMap((d) =>
      [value(d, unit, "adj"), value(d, unit, "raw")].filter(
        (v): v is number => v !== null,
      ),
    );
    const lo = Math.floor((Math.min(...vals) - step / 2) / step) * step;
    const hi = Math.ceil((Math.max(...vals) + step / 2) / step) * step;
    const t: number[] = [];
    for (let v = lo; v <= hi + 1e-9; v += step)
      t.push(Math.round(v * 100) / 100);
    return { domain: [t[0], t.at(-1)!] as [number, number], ticks: t };
  }, [days, unit]);

  const plotWidth =
    width - Y_AXIS_WIDTH - MARGIN.left - MARGIN.right - 2 * X_PADDING;
  const pxPerDay = plotWidth / Math.max(1, days.length - 1);
  // A tick needs about 32 px for "28/10"; skip ticks when days are narrower.
  const interval = Math.max(0, Math.ceil(32 / pxPerDay) - 1);
  const starts = CYCLE_STARTS.filter((s) => s >= days[0].date);

  function select(day: Day, kind: Kind) {
    hit.current = { day, kind };
    setPopup({ day, kind });
  }

  function onWrapperClick() {
    const h = hit.current;
    hit.current = null;
    setTaps((t) =>
      [
        {
          target,
          result: h ? `${fmtDate(h.day.date)} (${h.kind})` : "nothing",
          ok: h?.day.date === target,
        },
        ...t,
      ].slice(0, 30),
    );
  }

  function onChartClick(state: MouseHandlerDataParam) {
    const i = Number(state.activeIndex);
    const row = Number.isInteger(i) ? rows[i] : undefined;
    if (row && row.y !== null) select(row.day, "column");
  }

  // --- SVG pieces --------------------------------------------------------

  const dotMode = tapMode === "dot";

  function renderDayDot(p: DotItemDotProps): ReactNode {
    const row = p.payload as Row;
    if (p.cy == null || p.cx == null || row.y === null) return null;
    const ring = row.day.disturbances.length > 0;
    return (
      <circle
        cx={p.cx}
        cy={p.cy}
        r={ring ? 5 : 4.5}
        strokeWidth={ring ? 2 : 1}
        className={ring ? "fill-surface stroke-ink" : "fill-accent stroke-ink"}
        onClick={dotMode ? () => select(row.day, "dot") : undefined}
      />
    );
  }

  function renderGhost(p: DotItemDotProps): ReactNode {
    const row = p.payload as Row;
    if (p.cy == null || p.cx == null || row.ghost === null) return null;
    return (
      <circle
        cx={p.cx}
        cy={p.cy}
        r={3}
        className="fill-muted opacity-50"
        onClick={dotMode ? () => select(row.day, "ghost") : undefined}
      />
    );
  }

  // T-dot: an invisible 48 px circle under each dot and ghost.
  function hitDot(kind: Kind) {
    return function renderHit(p: DotItemDotProps): ReactNode {
      const row = p.payload as Row;
      if (p.cy == null || p.cx == null || p.value == null) return null;
      return (
        <circle
          cx={p.cx}
          cy={p.cy}
          r={24}
          className="fill-surface"
          fillOpacity={0}
          onClick={() => select(row.day, kind)}
        />
      );
    };
  }

  function renderXTick(p: XAxisTickContentProps): ReactNode {
    const date = String(p.payload.value);
    const day = days.find((d) => d.date === date);
    const cd = day?.cycleDay ?? (unknownStart ? "?" : "");
    return (
      <g transform={`translate(${Number(p.x)},${Number(p.y)})`}>
        <text dy={10} textAnchor="middle" className="fill-ink text-[10px]">
          {fmtDate(date)}
        </text>
        <text dy={23} textAnchor="middle" className="fill-muted text-[10px]">
          {cd}
        </text>
      </g>
    );
  }

  function renderStartMarker(p: LabelProps): ReactElement {
    const vb = p.viewBox as { x?: number; y?: number } | undefined;
    if (vb?.x == null || vb.y == null) return <g />;
    const { x, y } = vb;
    return (
      <polygon
        points={`${x - 5},${y - 9} ${x + 5},${y - 9} ${x},${y - 1}`}
        className="fill-accent stroke-ink"
        strokeWidth={1}
      />
    );
  }

  function runAudit() {
    const found = new Map<string, number>();
    box.current
      ?.querySelectorAll(
        "svg path, svg circle, svg line, svg rect, svg polygon, svg text",
      )
      .forEach((el) => {
        if (el.closest("clipPath")) return; // clip geometry is never painted
        const cs = getComputedStyle(el);
        for (const prop of ["fill", "stroke"] as const) {
          const v = cs[prop];
          if (v === "none") continue;
          const key = `${prop} ${v} → ${TOKEN_RGB[v] ?? "NOT A TOKEN"} (${el.tagName})`;
          found.set(key, (found.get(key) ?? 0) + 1);
        }
      });
    setAudit([...found].map(([k, n]) => `${n}× ${k}`).sort());
  }

  useEffect(() => {
    const t = setTimeout(runAudit, 2000); // after the line animation
    return () => clearTimeout(t);
  }, [rows, tapMode, width]);

  const targetOk = taps.filter((t) => t.target === target);

  // --- page --------------------------------------------------------------

  const btn = (on: boolean) =>
    `min-h-10 rounded border border-ink px-3 text-sm ${on ? "bg-accent" : "bg-surface"}`;

  return (
    <main className="mx-auto max-w-3xl p-2 text-sm">
      <h1 className="text-base font-semibold">Graph spike (fake data)</h1>

      <div ref={box} onClick={onWrapperClick} className="mt-2">
        {width > 0 && (
          <LineChart
            width={width}
            height={CHART_HEIGHT}
            data={rows}
            margin={MARGIN}
            onClick={dotMode ? undefined : onChartClick}
          >
            <XAxis
              dataKey="date"
              type="category"
              interval={interval}
              tick={renderXTick}
              tickLine={false}
              axisLine={{ stroke: INK }}
              height={36}
              padding={{ left: X_PADDING, right: X_PADDING }}
            />
            <YAxis
              type="number"
              domain={domain}
              ticks={ticks}
              width={Y_AXIS_WIDTH}
              tick={{ fill: MUTED, fontSize: 10 }}
              tickFormatter={(v: number) => v.toFixed(1)}
              axisLine={{ stroke: INK }}
              tickLine={{ stroke: INK }}
            />
            {!dotMode && <Tooltip content={() => null} cursor={false} />}
            {starts.map((s) => (
              <ReferenceLine
                key={s}
                x={s}
                stroke={INK}
                strokeDasharray="4 3"
                label={renderStartMarker}
              />
            ))}
            {dotMode && (
              <Line
                dataKey="hitY"
                stroke="none"
                dot={hitDot("dot")}
                activeDot={false}
                isAnimationActive={false}
              />
            )}
            {dotMode && ghosts && (
              <Line
                dataKey="hitGhost"
                stroke="none"
                dot={hitDot("ghost")}
                activeDot={false}
                isAnimationActive={false}
              />
            )}
            {runs.map((_, k) => (
              <Line
                key={`s${k}`}
                dataKey={`s${k}`}
                type="linear"
                stroke={INK}
                strokeWidth={2}
                connectNulls
                dot={false}
                activeDot={false}
                isAnimationActive={ANIMATE}
              />
            ))}
            {ghosts && (
              <Line
                dataKey="ghost"
                stroke="none"
                dot={renderGhost}
                activeDot={false}
                isAnimationActive={ANIMATE}
              />
            )}
            <Line
              dataKey="y"
              stroke="none"
              dot={renderDayDot}
              activeDot={false}
              isAnimationActive={ANIMATE}
            />
          </LineChart>
        )}
      </div>

      <div className="mt-2 flex flex-wrap gap-1">
        {RANGES.map((r) => (
          <button
            key={r}
            className={btn(range === r)}
            onClick={() => setRange(r)}
          >
            {r} d
          </button>
        ))}
        <button
          className={btn(unit === "F")}
          onClick={() => setUnit(unit === "C" ? "F" : "C")}
        >
          °{unit}
        </button>
        <button className={btn(ghosts)} onClick={() => setGhosts(!ghosts)}>
          ghosts {ghosts ? "on" : "off"}
        </button>
        <button
          className={btn(unknownStart)}
          onClick={() => setUnknownStart(!unknownStart)}
        >
          start {unknownStart ? "unknown" : "known"}
        </button>
        <button
          className={btn(tapMode === "column")}
          onClick={() => setTapMode(dotMode ? "column" : "dot")}
        >
          tap: {tapMode}
        </button>
      </div>
      <p className="mt-1 text-muted">
        window {window.innerWidth} · page {document.documentElement.scrollWidth}{" "}
        · dpr {window.devicePixelRatio} · animation {ANIMATE ? "on" : "off"} ·
        chart {width}px wide · {pxPerDay.toFixed(1)} px per day · tick every{" "}
        {interval + 1} · {runs.length} segments:{" "}
        {runs
          .map((r) => r.map((i) => fmtDate(days[i].date)).join("–"))
          .join(" | ")}
      </p>

      <section className="mt-3 rounded border border-ink p-2">
        <h2 className="font-semibold">
          Tap test: aim at{" "}
          <select
            className="border border-ink bg-surface"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
          >
            {days
              .filter((d) => d.rawMilli !== null)
              .map((d) => (
                <option key={d.date} value={d.date}>
                  {fmtDate(d.date)}
                </option>
              ))}
          </select>{" "}
          — {targetOk.filter((t) => t.ok).length}/{targetOk.length} correct
        </h2>
        <button className={btn(false)} onClick={() => setTaps([])}>
          clear taps
        </button>
        <ol className="mt-1 text-muted">
          {taps.slice(0, 10).map((t, i) => (
            <li key={i}>
              {t.ok ? "✓" : "✗"} aimed {fmtDate(t.target)} → {t.result}
            </li>
          ))}
        </ol>
      </section>

      <section className="mt-3 rounded border border-ink p-2">
        <h2 className="font-semibold">CSP violations: {violationCount}</h2>
        <button className={btn(false)} onClick={provokeViolation}>
          negative control (must add exactly 1)
        </button>
        <ul className="mt-1 break-all text-muted">
          {getViolations().map((v, i) => (
            <li key={i}>
              {v.directive} · {v.blocked || "inline"} · {v.sample}
            </li>
          ))}
        </ul>
      </section>

      <section className="mt-3 rounded border border-ink p-2">
        <h2 className="font-semibold">
          Colour audit (computed fill/stroke of every SVG shape)
        </h2>
        <button className={btn(false)} onClick={runAudit}>
          run again
        </button>
        <ul className="mt-1 break-all text-muted">
          {audit.map((a) => (
            <li key={a}>{a}</li>
          ))}
        </ul>
      </section>

      <section className="mt-3 rounded border border-ink p-2">
        <h2 className="font-semibold">Edge cases</h2>
        <ul>
          {CHECKLIST.map((c) => (
            <li key={c.id}>
              <b>{c.id}</b> {c.where}:{" "}
              <span className="text-muted">{c.expect}</span>
            </li>
          ))}
        </ul>
      </section>

      {popup && (
        <div className="fixed inset-x-0 bottom-0 border-t border-ink bg-surface p-3 shadow">
          <div className="mx-auto max-w-3xl">
            <p className="font-semibold">
              {fmtDate(popup.day.date)} · cycle day{" "}
              {popup.day.cycleDay ?? (unknownStart ? "?" : "—")} · via{" "}
              {popup.kind}
            </p>
            <p>
              Raw {value(popup.day, unit, "raw")?.toFixed(2)} °{unit} at{" "}
              {popup.day.measured} (fixed {hhmm(FIXED_MINUTES)})
            </p>
            {popup.day.adjusted && (
              <p>
                Adjusted {value(popup.day, unit, "adj")?.toFixed(2)} °{unit}{" "}
                (estimate) · {(popup.day.delta ?? 0) < 0 ? "late" : "early"}
              </p>
            )}
            {popup.day.disturbances.length > 0 && (
              <p>Disturbances: {popup.day.disturbances.join(", ")}</p>
            )}
            <div className="mt-2 flex gap-2">
              <button className={btn(true)} disabled>
                Show day
              </button>
              <button className={btn(false)} onClick={() => setPopup(null)}>
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </main>
  );
}

import { h } from 'preact';

interface ProgressRingProps {
  progress: number;
  size?: number;
  strokeWidth?: number;
}

export function ProgressRing({ progress, size = 200, strokeWidth = 8 }: ProgressRingProps) {
  const radius = (size - strokeWidth) / 2;
  const circumference = radius * 2 * Math.PI;
  const offset = circumference - (progress * circumference);

  return h('svg', {
    width: size,
    height: size,
    className: 'transform -rotate-90',
  },
    h('circle', {
      cx: size / 2,
      cy: size / 2,
      r: radius,
      fill: 'none',
      stroke: 'rgba(255, 255, 255, 0.2)',
      strokeWidth,
    }),
    h('circle', {
      cx: size / 2,
      cy: size / 2,
      r: radius,
      fill: 'none',
      stroke: 'white',
      strokeWidth,
      strokeLinecap: 'round',
      strokeDasharray: circumference,
      strokeDashoffset: offset,
      className: 'transition-all duration-200',
    })
  );
}

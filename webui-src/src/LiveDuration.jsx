import React, { useEffect, useState } from 'react';
import { formatDuration } from './util.js';

// LiveDuration ticks every second so "online for 1 分 03 秒" advances naturally
// between the 2-second status refreshes instead of jumping.
export default function LiveDuration({ since, fallback = '—' }) {
  const [, setTick] = useState(0);

  useEffect(() => {
    if (!since) return undefined;
    const timer = setInterval(() => setTick(value => value + 1), 1000);
    return () => clearInterval(timer);
  }, [since]);

  return <>{since ? formatDuration(since) : fallback}</>;
}

import React from 'react';
import { Typography as MuiTypography } from '@mui/material';

// MUI v9 Typography only handles variant/color/align/noWrap/gutterBottom.
// The old system props (fontWeight, display, letterSpacing, ...) are passed
// through as DOM attributes and ignored, so fold them into sx like Stack.jsx.
const SYSTEM_PROPS = [
  'display',
  'fontFamily',
  'fontSize',
  'fontStyle',
  'fontWeight',
  'letterSpacing',
  'lineHeight',
  'opacity',
  'overflow',
  'textAlign',
  'textOverflow',
  'textTransform',
  'whiteSpace',
  'wordBreak',
];

export default function Typography({ sx, ...props }) {
  const system = {};
  for (const key of SYSTEM_PROPS) {
    if (props[key] !== undefined) {
      system[key] = props[key];
      delete props[key];
    }
  }

  let mergedSx = sx;
  if (Object.keys(system).length > 0) {
    mergedSx = typeof sx === 'function' ? theme => ({ ...system, ...sx(theme) }) : { ...system, ...(sx || {}) };
  }

  return <MuiTypography {...props} sx={mergedSx} />;
}

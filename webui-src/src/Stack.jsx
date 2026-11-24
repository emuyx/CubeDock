import React from 'react';
import { Stack as MuiStack } from '@mui/material';

// MUI v9 Stack only handles direction/spacing/divider/useFlexGap. The older
// system props (justifyContent, alignItems, flexWrap, ...) are silently dropped
// instead of being converted to CSS, which left many toolbars misaligned.
// Fold them into sx so existing call sites keep their intended layout.
const SYSTEM_PROPS = [
  'alignContent',
  'alignItems',
  'alignSelf',
  'columnGap',
  'flexWrap',
  'gap',
  'justifyContent',
  'justifyItems',
  'justifySelf',
  'order',
  'rowGap',
];

export default function Stack({ sx, ...props }) {
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

  return <MuiStack {...props} sx={mergedSx} />;
}

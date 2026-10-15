import {createContext} from 'react';
// Presentation ownership only. Workspace remains the sole navigation decision
// maker; nested resource popovers must not place a second prompt above it.
export const LeavePromptContext=createContext(false);

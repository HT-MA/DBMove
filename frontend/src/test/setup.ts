import '@testing-library/jest-dom';

// antd v5 components rely on matchMedia; jsdom does not provide it.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
});

// required by antd responsive components
window.scrollTo = () => undefined;

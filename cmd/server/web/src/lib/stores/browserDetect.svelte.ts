// store whether we're on a narrow device
let smallDevice = $state(false);

export const onSmallDevice = () => smallDevice

// invoke this function as soon as window is available
export const detectSmallDevice = () => {
    // attach a media query listener to the window
    const mediaQuery = window.matchMedia('(width <= 640px)');

    // every time the media query matches or unmatches
    mediaQuery.addEventListener('change', ({ matches }) => {
      // set the state of our variable
      smallDevice = matches;
    });
}
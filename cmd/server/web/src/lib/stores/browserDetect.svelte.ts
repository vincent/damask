// store whether we're on a narrow device
let smallDevice = $state(false);

export const onSmallDevice = () => smallDevice

export const browserDetectStore = {
  get onSmallDevice() { return smallDevice },
  get onWideDevice() { return !smallDevice },

  detectSmallDevice() {
    // attach a media query listener to the window
    const mediaQuery = window.matchMedia('(width <= 640px)');

    // every time the media query matches or unmatches
    mediaQuery.addEventListener('change', ({ matches }) => {
      debugger;
      // set the state of our variable
      smallDevice = matches;
    });
  }
}
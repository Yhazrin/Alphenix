/**
 * Subtle full-viewport film grain (SVG feTurbulence), pointer-events none.
 * Kept intentionally light so it reads as texture, not noise.
 */
export function GrainOverlay() {
  const grainSvg =
    "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 256 256' width='256' height='256'%3E%3Cfilter id='a'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23a)'/%3E%3C/svg%3E\")";

  return (
    <div
      className="alphenix-grain pointer-events-none fixed inset-0 z-[1] opacity-[0.035] mix-blend-multiply dark:opacity-[0.055] dark:mix-blend-overlay"
      aria-hidden
      style={{
        backgroundImage: grainSvg,
        backgroundRepeat: "repeat",
      }}
    />
  );
}

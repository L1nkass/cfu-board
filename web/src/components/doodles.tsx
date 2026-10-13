// Декоративные «каракули» по углам — слой из исходного макета (doodles-layer).
export function Doodles() {
  return (
    <div
      aria-hidden
      className="pointer-events-none fixed inset-0 z-0 mx-auto h-screen w-full max-w-[1400px] overflow-hidden"
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src="/images/рыба.png" alt="" className="doodle absolute top-0 left-0" />
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src="/images/рука.png" alt="" className="doodle absolute right-0 bottom-0" />
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src="/images/корона.png" alt="" className="doodle absolute bottom-0 left-0" />
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src="/images/алмаз.png" alt="" className="doodle absolute top-0 right-0" />
    </div>
  );
}

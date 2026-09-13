export default function QuestionImage({ url }: { url?: string }) {
  if (!url) return null;
  return (
    <img
      src={url}
      alt="Gambar soal"
      loading="lazy"
      className="mb-2 max-h-80 w-full rounded-md border object-contain"
    />
  );
}

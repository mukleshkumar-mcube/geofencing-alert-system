import { ghostClass } from '../format'

export default function PageBar({ page, pages, from, to, total, onPrev, onNext }) {
  return (
    <div className="mt-3 flex flex-wrap items-center justify-end gap-2">
      <span className="text-xs text-white/60">{total === 0 ? '0 results' : `${from}–${to} of ${total}`}</span>
      <button className={ghostClass} type="button" disabled={page <= 1} onClick={onPrev}>Previous</button>
      <button className={ghostClass} type="button" disabled={page >= pages} onClick={onNext}>Next</button>
    </div>
  )
}

import { useQuery } from '@tanstack/react-query'

import { api } from '@/api/client'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const ALL = 'all'

/** Picks one device or all of them; `undefined` means all. */
export function DeviceSelect({ value, onChange }: { value?: string; onChange: (id: string | null) => void }) {
  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api.listDevices() })
  return (
    <Select value={value ?? ALL} onValueChange={(v) => onChange(v === ALL ? null : v)}>
      <SelectTrigger className="w-full sm:w-44" aria-label="Robot">
        <SelectValue />
      </SelectTrigger>
      <SelectContent position="popper">
        <SelectItem value={ALL}>All robots</SelectItem>
        {devices.data?.map((d) => (
          <SelectItem key={d.id} value={d.id}>
            {d.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

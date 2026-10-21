import { Button } from '@/components/ui/button'

export default function App() {
  return (
    <main className="flex min-h-svh flex-col items-start gap-4 p-8">
      <h1 className="text-2xl font-semibold">Cerebellum</h1>
      <p className="text-muted-foreground">Devices and their decisions will show up here.</p>
      <Button>Refresh</Button>
    </main>
  )
}

import { Navigate, Route, Routes } from 'react-router'

import { Layout } from '@/components/layout'
import { DevicesPage } from '@/pages/devices'
import { JournalPage } from '@/pages/journal'
import { OverviewPage } from '@/pages/overview'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<OverviewPage />} />
        <Route path="journal" element={<JournalPage />} />
        <Route path="devices" element={<DevicesPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

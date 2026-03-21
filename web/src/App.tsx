import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/layout/Layout'
import Dashboard from './pages/dashboard'
import Subscriptions from './pages/subscriptions'
import Events from './pages/events'
import EventAnalysis from './pages/event-analysis'
import Reports from './pages/reports'
import Chat from './pages/chat'
import Settings from './pages/settings'
import TermMapping from './pages/term-mapping'
import Traces from './pages/traces'
import TraceDetail from './pages/traces/detail'
import Knowledge from './pages/knowledge'
import KnowledgeDocs from './pages/knowledge/docs'
import KnowledgeChunks from './pages/knowledge/chunks'
import RagEval from './pages/rag-eval'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="subscriptions" element={<Subscriptions />} />
          <Route path="events" element={<Events />} />
          <Route path="events/analysis" element={<EventAnalysis />} />
          <Route path="reports" element={<Reports />} />
          <Route path="chat" element={<Chat />} />
          <Route path="settings" element={<Settings />} />
          <Route path="term-mapping" element={<TermMapping />} />
          <Route path="traces" element={<Traces />} />
          <Route path="traces/:traceId" element={<TraceDetail />} />
          <Route path="knowledge" element={<Knowledge />} />
          <Route path="knowledge/:baseId/docs" element={<KnowledgeDocs />} />
          <Route path="knowledge/:baseId/docs/:docId/chunks" element={<KnowledgeChunks />} />
          <Route path="rag-eval" element={<RagEval />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App

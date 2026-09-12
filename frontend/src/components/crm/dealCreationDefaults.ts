export type RevenueType = 'one_time' | 'monthly' | 'annual';
export function generatedDealName(company?: string, contact?: string) {
  const customer = company || contact;
  return customer ? `${customer} — New deal` : '';
}
export function recurringRevenue(amount: number | undefined, type: RevenueType = 'one_time') {
  if (amount == null || !Number.isFinite(amount) || type === 'one_time') return null;
  return type === 'monthly' ? { mrr: amount, arr: amount * 12 } : { mrr: amount / 12, arr: amount };
}
type Pipeline = { id: string; is_default: boolean; stages?: { id: string; position: number; probability: number }[] };
export function resolveDealStage(pipelines: Pipeline[], pipelineId?: string, stageId?: string) {
  const pipeline = pipelines.find(p => p.id === pipelineId) ?? pipelines.find(p => p.is_default) ?? pipelines[0];
  const stages = [...(pipeline?.stages ?? [])].sort((a,b) => a.position - b.position);
  const stage = stages.find(s => s.id === stageId) ?? stages[0];
  return { pipelineId: pipeline?.id ?? '', stageId: stage?.id ?? '', probability: stage?.probability ?? 0 };
}
export function revenueSuffix(type?: RevenueType) {
  return type === 'monthly' ? ' / mo' : type === 'annual' ? ' / yr' : '';
}
// Only sum comparable values. Mixed currencies or billing periods have no meaningful raw total.
export function comparableDealTotal(deals: { amount?: number; currency: string; revenue_type?: RevenueType }[]) {
  const valued = deals.filter(d => d.amount != null);
  if (!valued.length || new Set(valued.map(d => `${d.currency}:${d.revenue_type ?? 'one_time'}`)).size !== 1) return null;
  return `${valued[0].currency} ${new Intl.NumberFormat().format(valued.reduce((sum,d) => sum + (d.amount ?? 0), 0))}${revenueSuffix(valued[0].revenue_type)}`;
}

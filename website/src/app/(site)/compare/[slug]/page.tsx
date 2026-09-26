import { notFound } from 'next/navigation';
import { createPageMetadata } from '@/lib/metadata';
import { ComparePage } from '../ComparePage';
import { COMPETITORS, competitorSeo } from '../compare-data';
import '../compare.css';

export const dynamicParams = false;

export function generateStaticParams() {
  return COMPETITORS.map(competitor => ({ slug: competitor.slug }));
}

type Params = Promise<{ slug: string }>;

const find = (slug: string) => COMPETITORS.find(competitor => competitor.slug === slug);

export async function generateMetadata({ params }: { params: Params }) {
  const competitor = find((await params).slug);
  return competitor ? createPageMetadata(competitorSeo(competitor)) : {};
}

export default async function CompetitorComparison({ params }: { params: Params }) {
  const competitor = find((await params).slug);
  if (!competitor) notFound();
  return <ComparePage competitor={competitor} />;
}

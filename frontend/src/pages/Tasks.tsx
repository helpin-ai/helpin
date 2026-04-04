import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { CheckmarkSquare02Icon } from '@/lib/icons';
import { useTitle } from '@/hooks/useTitle';

export default function Tasks() {
  useTitle('Tasks');
  return (
    <div className="max-w-2xl mx-auto py-12 px-4">
      <Card>
        <CardHeader className="text-center">
          <div className="mx-auto h-12 w-12 rounded-full bg-primary/10 flex items-center justify-center mb-4">
            <CheckmarkSquare02Icon className="h-6 w-6 text-primary" />
          </div>
          <CardTitle className="text-xl">Tasks</CardTitle>
          <CardDescription>Coming soon</CardDescription>
        </CardHeader>
        <CardContent className="text-center">
          <p className="text-muted-foreground">
            Task management features are currently under development. Check back later for sprint task tracking, assignments, and progress updates.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

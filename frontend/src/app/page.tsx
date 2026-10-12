import { AddPostButton, EmptyFeedLink } from "@/components/add-post-button";
import { PostGrid } from "@/components/post-grid";

export default function HomePage() {
  return (
    <main className="container relative z-10 mx-auto mt-10 mb-16 px-5">
      <AddPostButton />
      <section className="mt-6">
        <PostGrid emptyHint={<EmptyFeedLink />} />
      </section>
    </main>
  );
}
